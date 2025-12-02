import { useState, useEffect } from "react";
import {
  Box,
  Grid,
  Typography,
  CircularProgress,
  Alert,
  Card,
  CardContent,
  Paper,
  useTheme,
} from "@mui/material";
import {
  NetworkCheck,
  Warning,
  Error,
  Hub,
  Block,
  PieChart as PieChartIcon,
} from "@mui/icons-material";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from "recharts";
import { format } from "date-fns";
import GrafanaEmbed from "../components/GrafanaEmbed";
import { metricsAPI } from "../services/api";

function StatCard({ title, value, icon: Icon, color, loading }) {
  const theme = useTheme();

  const colorMap = {
    primary: {
      main: theme.palette.primary.main,
      light: theme.palette.primary.light,
    },
    warning: {
      main: theme.palette.warning.main,
      light: theme.palette.warning.light,
    },
    error: { main: theme.palette.error.main, light: theme.palette.error.light },
    info: { main: theme.palette.info.main, light: theme.palette.info.light },
  };

  const colors = colorMap[color] || colorMap.primary;

  return (
    <Card
      sx={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        transition: "transform 0.2s, box-shadow 0.2s",
        "&:hover": {
          transform: "translateY(-4px)",
          boxShadow: 4,
        },
      }}
    >
      <CardContent>
        <Box
          display="flex"
          alignItems="center"
          justifyContent="space-between"
          mb={2}
        >
          <Box
            sx={{
              p: 1.5,
              borderRadius: 2,
              bgcolor: colors.light,
              color: colors.main,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Icon sx={{ fontSize: 20 }} />
          </Box>
          {loading && (
            <CircularProgress size={24} sx={{ color: colors.main }} />
          )}
          <Typography
            variant="h4"
            component="div"
            fontWeight="bold"
            gutterBottom
          >
            {loading
              ? "..."
              : typeof value === "number"
              ? value.toLocaleString()
              : value}
          </Typography>
        </Box>

        <Typography variant="h6" color="text.secondary">
          {title}
        </Typography>
      </CardContent>
    </Card>
  );
}

export default function Dashboard() {
  const [stats, setStats] = useState({
    totalFlows: 0,
    totalAlerts: 0,
    criticalAlerts: 0,
    tcpConnections: 0,
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [droppedFlowsData, setDroppedFlowsData] = useState([]);
  const [chartLoading, setChartLoading] = useState(true);
  const [alertTypesData, setAlertTypesData] = useState([]);
  const [alertTypesLoading, setAlertTypesLoading] = useState(true);

  const fetchStats = async () => {
    try {
      setError(null);
      const response = await metricsAPI.getPrometheusStats();
      setStats(response.data);
      setLoading(false);
    } catch (err) {
      console.error("Failed to fetch Prometheus stats:", err);
      setError(err.message || "Failed to load statistics");
      setLoading(false);
    }
  };

  const fetchDroppedFlowsTimeSeries = async () => {
    try {
      setChartLoading(true);
      const end = new Date();
      const start = new Date(end.getTime() - 1 * 60 * 60 * 1000); // Last 1 hour

      const response = await metricsAPI.getDroppedFlowsTimeSeries({
        start: start.toISOString(),
        end: end.toISOString(),
        step: "15s",
      });

      // Transform data for recharts
      const chartData = [];
      if (response.data.data && response.data.data.length > 0) {
        const series = response.data.data[0]; // Get first series
        if (series.values && series.values.length > 0) {
          // Group by timestamp and sum values from all namespaces
          const timeMap = new Map();

          response.data.data.forEach((s) => {
            s.values.forEach((point) => {
              const timestamp = point.timestamp;
              if (!timeMap.has(timestamp)) {
                timeMap.set(timestamp, {
                  timestamp,
                  time: format(new Date(timestamp * 1000), "HH:mm:ss"),
                  value: 0,
                });
              }
              timeMap.get(timestamp).value += point.value;
            });
          });

          chartData.push(
            ...Array.from(timeMap.values()).sort(
              (a, b) => a.timestamp - b.timestamp
            )
          );
        }
      }

      setDroppedFlowsData(chartData);
      setChartLoading(false);
    } catch (err) {
      console.error("Failed to fetch dropped flows time-series:", err);
      setChartLoading(false);
    }
  };

  const fetchAlertTypesStats = async () => {
    try {
      setAlertTypesLoading(true);
      const response = await metricsAPI.getAlertTypesStats();

      // Transform data for pie chart
      const pieData = [];
      if (response.data.byType && response.data.byType.length > 0) {
        response.data.byType.forEach((item) => {
          if (item.value > 0) {
            pieData.push({
              name: item.type || "unknown",
              value: item.value,
            });
          }
        });
      }

      setAlertTypesData(pieData);
      setAlertTypesLoading(false);
    } catch (err) {
      console.error("Failed to fetch alert types stats:", err);
      setAlertTypesLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();
    fetchDroppedFlowsTimeSeries();
    fetchAlertTypesStats();
    // Auto-refresh every 30 seconds
    const interval = setInterval(() => {
      fetchStats();
      fetchDroppedFlowsTimeSeries();
      fetchAlertTypesStats();
    }, 30000);
    return () => clearInterval(interval);
  }, []);

  return (
    <Box>
      <Grid container spacing={3}>
        {/* Statistics Cards */}
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="Total Flows"
            value={stats.totalFlows}
            icon={NetworkCheck}
            color="primary"
            loading={loading}
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="Total Alerts"
            value={stats.totalAlerts}
            icon={Warning}
            color="warning"
            loading={loading}
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="Critical Alerts"
            value={stats.criticalAlerts}
            icon={Error}
            color="error"
            loading={loading}
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="TCP Connections"
            value={stats.tcpConnections}
            icon={Hub}
            color="info"
            loading={loading}
          />
        </Grid>

        {/* Error Message */}
        {error && (
          <Grid item xs={12}>
            <Alert severity="error" onClose={() => setError(null)}>
              {error}
            </Alert>
          </Grid>
        )}

        {/* Dropped Flows Time-Series Chart */}
        <Grid item xs={12} md={6}>
          <Paper sx={{ p: 3 }}>
            <Box display="flex" alignItems="center" mb={2}>
              <Block sx={{ mr: 1, color: "error.main" }} />
              <Typography variant="h6" color="text.secondary">
                Dropped Flows Over Time
              </Typography>
            </Box>
            {chartLoading ? (
              <Box
                display="flex"
                justifyContent="center"
                alignItems="center"
                minHeight={300}
              >
                <CircularProgress />
              </Box>
            ) : droppedFlowsData.length === 0 ? (
              <Box
                display="flex"
                justifyContent="center"
                alignItems="center"
                minHeight={300}
              >
                <Typography color="text.secondary">
                  No dropped flows data
                </Typography>
              </Box>
            ) : (
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={droppedFlowsData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis
                    dataKey="time"
                    tick={{ fontSize: 12 }}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    tick={{ fontSize: 12 }}
                    label={{
                      value: "Dropped Flows",
                      angle: -90,
                      position: "insideLeft",
                    }}
                  />
                  <Tooltip
                    formatter={(value) => [value.toFixed(0), "Dropped Flows"]}
                    labelFormatter={(label) => `Time: ${label}`}
                  />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="value"
                    stroke="#d32f2f"
                    strokeWidth={2}
                    dot={false}
                    name="Dropped Flows"
                    isAnimationActive={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            )}
          </Paper>
        </Grid>

        {/* Alert Types Pie Chart */}
        <Grid item xs={12} md={6}>
          <Paper sx={{ p: 3 }}>
            <Box display="flex" alignItems="center" mb={2}>
              <PieChartIcon sx={{ mr: 1, color: "warning.main" }} />
              <Typography variant="h6" color="text.secondary">
                Alert Types
              </Typography>
            </Box>
            {alertTypesLoading ? (
              <Box
                display="flex"
                justifyContent="center"
                alignItems="center"
                minHeight={300}
              >
                <CircularProgress />
              </Box>
            ) : alertTypesData.length === 0 ? (
              <Box
                display="flex"
                justifyContent="center"
                alignItems="center"
                minHeight={300}
              >
                <Typography color="text.secondary">No alert data</Typography>
              </Box>
            ) : (
              <ResponsiveContainer width="100%" height={300}>
                <PieChart>
                  <Pie
                    data={alertTypesData}
                    cx="50%"
                    cy="50%"
                    labelLine={false}
                    label={({ name }) => name}
                    outerRadius={80}
                    fill="#8884d8"
                    dataKey="value"
                    stroke="none"
                  >
                    {alertTypesData.map((entry, index) => {
                      const colors = [
  '#7EB26D', // green
  '#EAB839', // yellow
  '#6ED0E0', // cyan
  '#EF843C', // orange
  '#E24D42', // red
  '#1F78C1', // blue
  '#BA43A9'  // purple
]


                      return (
                        <Cell
                          key={`cell-${index}`}
                          fill={colors[index % colors.length]}
                          stroke="none"
                        />
                      );
                    })}
                  </Pie>
                  <Tooltip
                    formatter={(value) => [value.toFixed(0), "Alerts"]}
                  />
                  <Legend />
                </PieChart>
              </ResponsiveContainer>
            )}
          </Paper>
        </Grid>

        {/* Grafana Dashboard */}
        <Grid item xs={12}>
          <GrafanaEmbed
            dashboardUid={
              import.meta.env.VITE_GRAFANA_DASHBOARD_UID || "hubble-guard"
            }
            height="800px"
            title="Grafana Hubble Network Monitoring"
          />
        </Grid>
      </Grid>
    </Box>
  );
}
