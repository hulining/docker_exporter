package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/hulining/docker_exporter/collector"

	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var (
	Version      string
	gitCommit    string
	gitTreeState = ""                     // state of git tree, either "clean" or "dirty"
	buildDate    = "1970-01-01T00:00:00Z" // build date, output of $(date +'%Y-%m-%dT%H:%M:%S')
)

func versionPrint() string {
	return fmt.Sprintf(`Name: %s
Version: %s
CommitID: %s
GitTreeState: %s
BuildDate: %s
GoVersion: %s
Compiler: %s
Platform: %s/%s
`, collector.Name(), Version, gitCommit, gitTreeState, buildDate, runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH)
}

func LogInit(level, file string) error {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	le, err := log.ParseLevel(level)
	if err != nil {
		return err
	}
	log.SetLevel(le)

	if file != "" {
		f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		log.SetOutput(f)
	}
	return nil
}

func main() {
	listenAddress := flag.String("web.listen-address", ":9417", "Address to listen on for web interface and telemetry.")
	metricsPath := flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	logLevel := flag.String("log-level", "info", "The logging level:[debug, info, warn, error, fatal]")
	logFile := flag.String("log-output", "", "The file which log to, default stdout")
	versionP := flag.Bool("version", false, "Print version info")

	scraperFlags := map[collector.Scraper]*bool{}
	for scraper, enabledByDefault := range collector.Scrapers {
		defaultOn := false
		if enabledByDefault {
			defaultOn = true
		}
		f := flag.Bool("collect."+scraper.Name(), defaultOn, scraper.Help())
		scraperFlags[scraper] = f
	}

	flag.Parse()

	if *versionP {
		fmt.Print(versionPrint())
		return
	}

	if err := LogInit(*logLevel, *logFile); err != nil {
		log.Fatal(errors.Wrap(err, "set log level error"))
	}

	// Register only scrapers enabled by flag.
	enabledScrapers := []collector.Scraper{}
	for scraper, enabled := range scraperFlags {
		if *enabled {
			log.Info("Scraper enabled ", scraper.Name())
			enabledScrapers = append(enabledScrapers, scraper)
		}
	}

	exporter, err := collector.New(collector.NewMetrics(), enabledScrapers, client.FromEnv)
	if err != nil {
		log.Fatal(err)
	}

	prometheus.MustRegister(exporter)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
            <head><title>` + collector.Name() + `</title></head>
            <body>
            <h1><a style="text-decoration:none" href=''>` + collector.Name() + `</a></h1>
            <p><a href='` + *metricsPath + `'>metrics</a></p>
            <h2>Build</h2>
            <pre>` + versionPrint() + `</pre>
            </body>
            </html>`))
	})

	http.Handle(*metricsPath, promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{
				ErrorLog: log.StandardLogger(),
			},
		),
	),
	)

	http.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok")
	})

	log.Info("Listening on address ", *listenAddress)

	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatal(err)
	}
}
