import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:5001/api/v1'
const WS_BASE_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:5001/api/v1'

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Flows API
export const flowsAPI = {
  getAll: (params = {}) => api.get('/flows', { params }),
  getById: (id) => api.get(`/flows/${id}`),
  getStats: () => api.get('/flows/stats'),
}

// Alerts API
export const alertsAPI = {
  getAll: (params = {}) => api.get('/alerts', { params }),
  getById: (id) => api.get(`/alerts/${id}`),
  getTimeline: (params = {}) => api.get('/alerts/timeline', { params }),
}

// Rules API
export const rulesAPI = {
  getAll: () => api.get('/rules'),
  getById: (id) => api.get(`/rules/${id}`),
  update: (id, data) => api.put(`/rules/${id}`, data),
  getStats: () => api.get('/rules/stats'),
}

// Metrics API
export const metricsAPI = {
  getPrometheus: (query) => api.get('/metrics/prometheus', { params: { query } }),
  getStats: () => api.get('/metrics/stats'),
}

// WebSocket connections
export const createWebSocket = (endpoint) => {
  const wsUrl = `${WS_BASE_URL}${endpoint}`
  return new WebSocket(wsUrl)
}

export default api

