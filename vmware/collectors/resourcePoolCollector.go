package vmwareCollectors

import (
  "context"
  "flag"
  "fmt"
  "log/slog"
  "sync"
  "time"

  "github.com/prezhdarov/prometheus-exporter/collector"
  "github.com/prometheus/client_golang/prometheus"
  "github.com/vmware/govmomi/performance"
  "github.com/vmware/govmomi/view"
  "github.com/vmware/govmomi/vim25"
  "github.com/vmware/govmomi/vim25/mo"
  "github.com/vmware/govmomi/vim25/types"
)

const (
  resourcePoolSubsystem = "resourcepool"
)

var resourcePoolCollectorFlag = flag.Bool(
  fmt.Sprintf("collector.%s", resourcePoolSubsystem),
  collector.DefaultEnabled,
  fmt.Sprintf("Enable the %s collector (default: %v)", resourcePoolSubsystem, collector.DefaultEnabled),
)

var rpCounters = []string{
  "cpu.usage.average",
  "cpu.usagemhz.average",
  "cpu.entitlement.average",
  "cpu.demand.average",
  "mem.usage.average",
  "mem.consumed.average",
  // adicione outros que quiser
}

func init() {
  collector.RegisterCollector("resourcepool", resourcePoolCollectorFlag, NewResourcePoolCollector)
}

type resourcePoolCollector struct {
  logger *slog.Logger
}

func NewResourcePoolCollector(logger *slog.Logger) (collector.Collector, error) {
  return &resourcePoolCollector{logger: logger}, nil
}

func (c *resourcePoolCollector) Update(
  ch chan<- prometheus.Metric,
  namespace string,
  clientAPI collector.ClientAPI,
  loginData map[string]interface{},
  params map[string]string,
) error {
  ctx := loginData["ctx"].(context.Context)
  viewMgr := loginData["view"].(*view.Manager)
  client := loginData["client"].(*vim25.Client)

  // 1. fetch ResourcePool objects
  var pools []mo.ResourcePool
  err := fetchProperties(
    ctx, viewMgr, client,
    []string{"ResourcePool"},
    []string{"name", "parent"},
    &pools,
    c.logger,
  )
  if err != nil {
    return err
  }

  // 2. montar targetRefs e targetNames
  var poolRefs []types.ManagedObjectReference
  targetNames := make(map[string]string)
  for _, rp := range pools {
    poolRefs = append(poolRefs, rp.Self)
    targetNames[rp.Self.Value] = rp.Name
    // emitir métrica info
    ch <- prometheus.MustNewConstMetric(
      prometheus.NewDesc(
        prometheus.BuildFQName(namespace, resourcePoolSubsystem, "info"),
        "ResourcePool info metric",
        nil,
        map[string]string{"rpmo": rp.Self.Value, "rp": rp.Name, "vcenter": loginData["target"].(string)},
      ),
      prometheus.GaugeValue, 1.0,
    )
  }

  if len(poolRefs) == 0 {
    return nil
  }

  // 3. chamar scrapePerformance para ResourcePool
  scrapePerformance(
    ctx, ch, c.logger,
    loginData["samples"].(int32),
    loginData["interval"].(int32),
    loginData["perf"].(*performance.Manager),
    loginData["target"].(string),
    "ResourcePool",                      // moType
    namespace,
    resourcePoolSubsystem,
    "*",                                 // instância wildcard, se quiser por instância
    rpCounters,
    loginData["counters"].(map[string]*types.PerfCounterInfo),
    poolRefs,
    targetNames,
  )

  return nil
}
