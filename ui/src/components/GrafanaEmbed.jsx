import { useState, useEffect } from 'react'
import { Box, Paper, Typography, Alert, CircularProgress, IconButton } from '@mui/material'
import { Refresh, OpenInNew } from '@mui/icons-material'

// Grafana Logo Image Component
const GrafanaIcon = ({ sx }) => (
  <Box
    component="img"
    src="https://grafana.com/static/img/menu/grafana2.svg"
    alt="Grafana"
    sx={{
      width: 24,
      height: 24,
      ...sx,
    }}
    onError={(e) => {
      // Fallback to a simple colored box if image fails to load
      e.target.style.display = 'none'
    }}
  />
)

const GRAFANA_URL = import.meta.env.VITE_GRAFANA_URL || 'http://localhost:3000'
const USE_PROXY = import.meta.env.VITE_GRAFANA_USE_PROXY === 'true' // Default to false - use direct URL
const GRAFANA_USER = import.meta.env.VITE_GRAFANA_USER || 'admin'
const GRAFANA_PASSWORD = import.meta.env.VITE_GRAFANA_PASSWORD || 'admin'

export default function GrafanaEmbed({ dashboardUid, panelId, height = '600px', title }) {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [iframeKey, setIframeKey] = useState(0)

  const getEmbedUrl = () => {
    if (!dashboardUid) {
      return null
    }

    let baseUrl
    if (USE_PROXY && import.meta.env.DEV) {
      baseUrl = '/grafana'
    } else {
      baseUrl = GRAFANA_URL.replace(/\/$/, '')
    }

    let url = `${baseUrl}/d/${dashboardUid}`
    
    // Build query parameters
    const params = new URLSearchParams()
    
    if (panelId) {
      params.append('viewPanel', panelId)
    }
    
    // Always enable kiosk mode for cleaner display
    params.append('kiosk', 'tv')
    
    // Add time range and refresh
    params.append('from', 'now-1h')
    params.append('to', 'now')
    params.append('refresh', '30s')
    
    url += `?${params.toString()}`

    return url
  }

  const embedUrl = getEmbedUrl()

  const handleRefresh = () => {
    setIframeKey(prev => prev + 1)
    setLoading(true)
  }

  const handleLoad = () => {
    setLoading(false)
    setError(null)
  }

  const handleError = (e) => {
    setLoading(false)
    console.error('Grafana iframe error:', e)
    setError('Failed to load Grafana dashboard. Please check if Grafana is running and accessible.')
  }

  useEffect(() => {
    if (!embedUrl) return

    const timeout = setTimeout(() => {
      if (loading) {
        setLoading(false)
        setError(`Grafana dashboard is taking too long to load. Please check if Grafana is running at ${GRAFANA_URL}`)
      }
    }, 10000)

    return () => clearTimeout(timeout)
  }, [loading, embedUrl])

  const openInNewTab = () => {
    if (embedUrl) {
      window.open(embedUrl, '_blank')
    }
  }

  if (!dashboardUid) {
    return (
      <Alert severity="warning">
        Grafana dashboard UID is not configured. Please set VITE_GRAFANA_URL and dashboard UID.
      </Alert>
    )
  }

  return (
    <Paper sx={{ p: 2, position: 'relative' }}>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Box display="flex" alignItems="center">
          <GrafanaIcon sx={{ mr: 1 }} />
          <Typography variant="h6" color="text.secondary">
            {title || 'Grafana Dashboard'}
          </Typography>
        </Box>
        <Box>
          <IconButton size="small" onClick={handleRefresh} title="Refresh">
            <Refresh />
          </IconButton>
          <IconButton size="small" onClick={openInNewTab} title="Open in new tab">
            <OpenInNew />
          </IconButton>
        </Box>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {loading && (
        <Box display="flex" justifyContent="center" alignItems="center" minHeight={height}>
          <CircularProgress />
        </Box>
      )}

      <Box
        sx={{
          position: 'relative',
          width: '100%',
          height: height,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
          overflow: 'hidden',
          bgcolor: 'background.paper',
        }}
      >
        {embedUrl && (
          <iframe
            key={iframeKey}
            src={embedUrl}
            title={title || 'Grafana Dashboard'}
            style={{
              width: '100%',
              height: '100%',
              border: 'none',
              display: loading ? 'none' : 'block',
            }}
            onLoad={handleLoad}
            onError={handleError}
            allow="fullscreen"
            sandbox={USE_PROXY && import.meta.env.DEV ? undefined : "allow-same-origin allow-scripts allow-popups allow-forms"}
          />
        )}
      </Box>

      {embedUrl && (
        <Typography variant="caption" color="textSecondary" sx={{ mt: 1, display: 'block' }}>
          Grafana URL: {embedUrl.replace(/\/\/.*@/, '//***:***@')}
        </Typography>
      )}
    </Paper>
  )
}

