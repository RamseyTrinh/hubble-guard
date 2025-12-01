import { Box, Grid, Typography } from '@mui/material'
import GrafanaEmbed from '../components/GrafanaEmbed'

export default function Dashboard() {
  return (
    <Box>
      <Grid container spacing={3}>
        <Grid item xs={12}>
          <GrafanaEmbed
            dashboardUid={import.meta.env.VITE_GRAFANA_DASHBOARD_UID || 'hubble-guard'}
            height="800px"
            title="Hubble Network Monitoring Dashboard"
          />
        </Grid>
      </Grid>
    </Box>
  )
}

