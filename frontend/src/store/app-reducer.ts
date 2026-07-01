import { combineReducers } from '@reduxjs/toolkit';
import { api } from '../api';
import { goServiceApi } from '../api/go-service-api';
import { authReducer } from '@/domains/auth/slice';

export const appReducer = combineReducers({
  [api.reducerPath]: api.reducer,
  [goServiceApi.reducerPath]: goServiceApi.reducer,
  auth: authReducer
});
