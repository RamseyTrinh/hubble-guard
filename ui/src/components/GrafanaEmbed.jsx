import { useState, useEffect } from 'react'
import { Box, Paper, Typography, Alert, CircularProgress, IconButton } from '@mui/material'
import { Refresh, OpenInNew } from '@mui/icons-material'

const GRAFANA_URL = import.meta.env.VITE_GRAFANA_URL || 'http://localhost:3000'
const USE_PROXY = import.meta.env.VITE_GRAFANA_USE_PROXY === 'true' // Default to false - use direct URL
const GRAFANA_USER = import.meta.env.VITE_GRAFANA_USER || 'admin'
const GRAFANA_PASSWORD = import.meta.env.VITE_GRAFANA_PASSWORD || 'admin'

export default function GrafanaEmbed({ dashboardUid, panelId, height = '600px', title }) {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [iframeKey, setIframeKey] = useState(0)

  // Generate Grafana embed URL
  const getEmbedUrl = () => {
    if (!dashboardUid) {
      return null
    }

    // Use direct URL by default (Grafana needs to allow embedding)
    // Proxy mode can be enabled but Grafana needs subpath config
    let baseUrl
    if (USE_PROXY && import.meta.env.DEV) {
      // Use Vite proxy (requires Grafana subpath config)
      baseUrl = '/grafana'
    } else {
      // Use direct URL (simpler, requires Grafana to allow embedding)
      baseUrl = GRAFANA_URL.replace(/\/$/, '')
    }

    let url = `${baseUrl}/d/${dashboardUid}`
    
    // Add panel ID if specified
    if (panelId) {
      url += `?viewPanel=${panelId}`
    } else {
      url += '?kiosk=tv' // TV mode - hides header and sidebar
    }

    // Add time range and refresh
    url += `&from=now-1h&to=now&refresh=30s`

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

    // Set a timeout to detect if iframe fails to load
    const timeout = setTimeout(() => {
      if (loading) {
        setLoading(false)
        setError(`Grafana dashboard is taking too long to load. Please check if Grafana is running at ${GRAFANA_URL}`)
      }
    }, 10000) // 10 seconds timeout

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
        <Typography variant="h6">
          {title || 'Grafana Dashboard'}
        </Typography>
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
            // Remove sandbox to allow cross-origin iframe (only for development)
            sandbox={USE_PROXY && import.meta.env.DEV ? undefined : "allow-same-origin allow-scripts allow-popups allow-forms"}
          />
        )}
      </Box>

      {embedUrl && (
        <Typography variant="caption" color="textSecondary" sx={{ mt: 1, display: 'block' }}>
          Grafana URL: {embedUrl.replace(/\/\/.*@/, '//***:***@')} {/* Hide credentials */}
        </Typography>
      )}
    </Paper>
  )
}

