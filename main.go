package main

import (
    "os"

    "github.com/LucaChot/pronto-framework/plugin"
    scheduler "k8s.io/kubernetes/cmd/kube-scheduler/app"
)

func main() {
    cmd := scheduler.NewSchedulerCommand(
        scheduler.WithPlugin(plugin.Name, plugin.New),
    )

    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
