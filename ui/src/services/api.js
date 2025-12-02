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
  getPrometheusStats: () => api.get('/metrics/prometheus/stats'),
  getDroppedFlowsTimeSeries: (params = {}) => api.get('/metrics/prometheus/dropped-flows/timeseries', { params }),
  getAlertTypesStats: () => api.get('/metrics/prometheus/alert-types'),
}

export const createWebSocket = (endpoint) => {
  const wsUrl = `${WS_BASE_URL}${endpoint}`
  return new WebSocket(wsUrl)
}

export default api

