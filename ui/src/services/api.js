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

export const metricsAPI = {
  getPrometheus: (query) => api.get('/metrics/prometheus', { params: { query } }),
  getStats: () => api.get('/metrics/stats'),
}

export const createWebSocket = (endpoint) => {
  const wsUrl = `${WS_BASE_URL}${endpoint}`
  return new WebSocket(wsUrl)
}

export default api

