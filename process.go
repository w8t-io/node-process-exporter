package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"
)

const (
	namespace = "node"
	subsystem = "process"
)

// 定义指标的标签
var processLabels = []string{"name", "pid", "cmd", "user"}

// ProcessCollector 实现了 prometheus.Collector 接口
type ProcessCollector struct {
	CPU             *prometheus.Desc
	Memory          *prometheus.Desc
	OpenFiles       *prometheus.Desc
	ReadBytesTotal  *prometheus.Desc
	WriteBytesTotal *prometheus.Desc
}

// NewProcessCollector 创建一个新的 ProcessCollector
func NewProcessCollector() *ProcessCollector {
	return &ProcessCollector{
		CPU: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "cpu_usage_percent"),
			"Process CPU usage percentage.",
			processLabels,
			nil,
		),
		Memory: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "memory_usage_percent"),
			"Process memory usage percentage.",
			processLabels,
			nil,
		),
		OpenFiles: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "open_files_count"),
			"Number of open files by the process.",
			processLabels,
			nil,
		),
		ReadBytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "read_bytes_total"),
			"Total number of bytes read by the process.",
			processLabels,
			nil,
		),
		WriteBytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "write_bytes_total"),
			"Total number of bytes written by the process.",
			processLabels,
			nil,
		),
	}
}

// Describe 将所有指标的描述符发送到提供的 channel
func (pc *ProcessCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- pc.CPU
	ch <- pc.Memory
	ch <- pc.OpenFiles
	ch <- pc.ReadBytesTotal
	ch <- pc.WriteBytesTotal
}

// Collect 收集所有进程的指标数据
func (pc *ProcessCollector) Collect(ch chan<- prometheus.Metric) {
	processes, err := process.Processes()
	if err != nil {
		logrus.Errorf("Failed to get processes: %v", err)
		return
	}

	for _, proc := range processes {
		pid := proc.Pid
		name, err := proc.Name()
		if err != nil {
			logrus.Debugf("Failed to get name for PID %d, err: %v", pid, err)
			continue
		}

		cmdline, err := proc.Cmdline()
		if err != nil {
			logrus.Debugf("Failed to get cmdline for PID %d (%s), err: %v", pid, name, err)
			cmdline = ""
		}

		user, err := proc.Username()
		if err != nil {
			logrus.Debugf("Failed to get username for PID %d (%s), err: %v", pid, name, err)
			user = "unknown"
		}

		// 创建标签值
		labelValues := []string{name, strconv.Itoa(int(pid)), cmdline, user}

		// 获取并注册 CPU 指标
		if cpuPercent, err := proc.CPUPercent(); err == nil {
			if cpuPercent > 0 {
				ch <- prometheus.MustNewConstMetric(pc.CPU, prometheus.GaugeValue, cpuPercent, labelValues...)
			}
		} else {
			logrus.Debugf("Failed to get CPU usage for PID %d (%s), err: %v", pid, name, err)
		}

		// 获取并注册内存指标
		if memPercent, err := getProcMemoryPercent(proc); err == nil {
			if memPercent > 0 {
				ch <- prometheus.MustNewConstMetric(pc.Memory, prometheus.GaugeValue, memPercent, labelValues...)
			}
		} else {
			logrus.Debugf("Failed to get memory usage for PID %d (%s), err: %v", pid, name, err)
		}

		// 获取并注册文件打开数指标
		if openFiles, err := proc.OpenFiles(); err == nil {
			count := len(openFiles)
			if count > 0 {
				ch <- prometheus.MustNewConstMetric(pc.OpenFiles, prometheus.GaugeValue, float64(count), labelValues...)
			}
		} else {
			logrus.Debugf("Failed to get open files for PID %d (%s), err: %v", pid, name, err)
		}

		// 获取并注册磁盘读写
		if ioCounters, err := proc.IOCounters(); err == nil {
			ch <- prometheus.MustNewConstMetric(pc.ReadBytesTotal, prometheus.CounterValue, float64(ioCounters.ReadBytes), labelValues...)
			ch <- prometheus.MustNewConstMetric(pc.WriteBytesTotal, prometheus.CounterValue, float64(ioCounters.WriteBytes), labelValues...)
		} else {
			logrus.Debugf("Failed to get IO counters for PID %d (%s), err: %v", pid, name, err)
		}
	}
}

// getProcMemoryPercent 计算单个进程的内存使用百分比
func getProcMemoryPercent(proc *process.Process) (float64, error) {
	procMem, err := proc.MemoryInfo()
	if err != nil {
		return 0, err
	}

	nodeMem, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}

	// 进程内存使用率 = (进程使用的物理内存 / 节点总物理内存) * 100
	return (float64(procMem.RSS) / float64(nodeMem.Total)) * 100.0, nil
}

func main() {
	var (
		port  int
		level logrus.Level
	)
	ll := os.Getenv("LOG_LEVEL")
	if ll == "debug" {
		level = logrus.DebugLevel
	} else {
		level = logrus.InfoLevel
	}

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 9002
	} else if port < 1024 {
		logrus.Fatalf("Invalid port number: %d. Please use a port number greater than or equal to 1024.", port)
	}

	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetLevel(level)

	// 创建自定义采集器并注册
	procCollector := NewProcessCollector()
	registry := prometheus.NewRegistry()
	registry.MustRegister(procCollector)

	// 创建 HTTP 处理器
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})

	http.Handle("/metrics", handler)

	addr := fmt.Sprintf(":%d", port)
	logrus.Infof("Service started! Listening on %s", addr)

	// 启动 HTTP 服务
	if err := http.ListenAndServe(addr, nil); err != nil {
		logrus.Fatalf("Failed to start HTTP server: %v", err)
	}
}
