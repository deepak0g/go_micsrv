import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import Cookies from 'js-cookie';

const baseQuery = fetchBaseQuery({
  baseUrl: `${import.meta.env.VITE_GO_SERVICE_URL || 'http://localhost:8080'}/api/v1`,
  prepareHeaders: (headers) => {
    const csrfToken = Cookies.get('csrfToken');
    if (csrfToken) {
      headers.set('x-csrf-token', csrfToken);
    }
    return headers;
  },
  credentials: 'include'
});

export const goServiceApi = createApi({
  reducerPath: 'goServiceApi',
  baseQuery,
  tagTypes: [],
  endpoints: () => ({})
});
