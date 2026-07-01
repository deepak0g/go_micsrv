package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jung-kurt/gofpdf"

	"github.com/rs/cors"
)

// NOTE - can be set via env
const (
	backendBaseURL = "http://localhost:5000/api/v1"
	adminUsername  = "admin@school-admin.com"
	adminPassword  = "3OU4zn3q6Zh9"
)

type authSession struct {
	AccessToken  string
	RefreshToken string
	CsrfToken    string
}

func (a authSession) cookieHeader() string {
	return fmt.Sprintf("accessToken=%s; refreshToken=%s; csrfToken=%s", a.AccessToken, a.RefreshToken, a.CsrfToken)
}

func parseSessionFromResponse(resp *http.Response, current authSession) authSession {
	next := current
	for _, c := range resp.Cookies() {
		switch c.Name {
		case "accessToken":
			next.AccessToken = c.Value
		case "refreshToken":
			next.RefreshToken = c.Value
		case "csrfToken":
			next.CsrfToken = c.Value
		}
	}
	return next
}

func loginToBackend(client *http.Client) (authSession, error) {
	payload := map[string]string{
		"username": adminUsername,
		"password": adminPassword,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return authSession{}, err
	}

	req, err := http.NewRequest("POST", backendBaseURL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return authSession{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return authSession{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return authSession{}, fmt.Errorf("login failed: %s", string(errBody))
	}

	session := parseSessionFromResponse(resp, authSession{})
	if session.AccessToken == "" || session.RefreshToken == "" || session.CsrfToken == "" {
		return authSession{}, fmt.Errorf("login succeeded but auth cookies are missing")
	}

	return session, nil
}

func refreshBackendSession(client *http.Client, session authSession) (authSession, error) {
	req, err := http.NewRequest("GET", backendBaseURL+"/auth/refresh", nil)
	if err != nil {
		return authSession{}, err
	}
	req.Header.Set("Cookie", session.cookieHeader())
	req.Header.Set("x-csrf-token", session.CsrfToken)

	resp, err := client.Do(req)
	if err != nil {
		return authSession{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return authSession{}, fmt.Errorf("refresh failed: %s", string(errBody))
	}

	next := parseSessionFromResponse(resp, session)
	if next.AccessToken == "" || next.CsrfToken == "" {
		return authSession{}, fmt.Errorf("refresh succeeded but tokens are missing")
	}

	return next, nil
}

func getString(data map[string]interface{}, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return "-"
	}
	return fmt.Sprintf("%v", value)
}

func generateStudentPDF(studentID string, data map[string]interface{}) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Student Report", false)
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(190, 10, "Student Report")
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(190, 8, fmt.Sprintf("Report ID: %s", studentID))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Student Name: %s", getString(data, "name")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Email: %s", getString(data, "email")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Class: %s", getString(data, "class")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Section: %s", getString(data, "section")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Roll: %s", getString(data, "roll")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Phone: %s", getString(data, "phone")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("DOB: %s", getString(data, "dob")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Father Name: %s", getString(data, "fatherName")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Father Phone: %s", getString(data, "fatherPhone")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Mother Name: %s", getString(data, "motherName")))
	pdf.Ln(8)
	pdf.Cell(190, 8, fmt.Sprintf("Guardian Name: %s", getString(data, "guardianName")))
	pdf.Ln(8)
	pdf.MultiCell(190, 8, fmt.Sprintf("Current Address: %s", getString(data, "currentAddress")), "", "L", false)
	pdf.MultiCell(190, 8, fmt.Sprintf("Permanent Address: %s", getString(data, "permanentAddress")), "", "L", false)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func main() {
	r := chi.NewRouter()

	r.Get("/api/v1/students/{id}/report", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		client := &http.Client{}

		session, err := loginToBackend(client)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		backendURL := fmt.Sprintf("%s/students/%s", backendBaseURL, id)

		req, err := http.NewRequestWithContext(r.Context(), "GET", backendURL, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Cookie", session.cookieHeader())
		req.Header.Set("x-csrf-token", session.CsrfToken)

		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()

			session, err = refreshBackendSession(client, session)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			retryReq, err := http.NewRequestWithContext(r.Context(), "GET", backendURL, nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			retryReq.Header.Set("Cookie", session.cookieHeader())
			retryReq.Header.Set("x-csrf-token", session.CsrfToken)

			resp, err = client.Do(retryReq)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if resp.StatusCode != http.StatusOK {
			http.Error(w, string(body), resp.StatusCode)
			return
		}

		var studentData map[string]interface{}
		if err := json.Unmarshal(body, &studentData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		pdfBytes, err := generateStudentPDF(id, studentData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=student-%s-report.pdf", id))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
		w.WriteHeader(http.StatusOK)
		w.Write(pdfBytes)
	})

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	handler := c.Handler(r)

	fmt.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		fmt.Println("Server error:", err)
	}
}
