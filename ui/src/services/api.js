import axios from 'axios'

// Use relative path in production (nginx will proxy), or absolute URL in development
const API_BASE_URL = import.meta.env.VITE_API_URL || (import.meta.env.PROD ? '/api/v1' : 'http://localhost:5001/api/v1')
const WS_BASE_URL = import.meta.env.VITE_WS_URL || (import.meta.env.PROD ? `ws://${window.location.host}/api/v1` : 'ws://localhost:5001/api/v1')

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

export const flowsAPI = {
  getAll: (params = {}) => api.get('/flows', { params }),
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

// Export WS_BASE_URL for direct use if needed
export { WS_BASE_URL }

export default api

