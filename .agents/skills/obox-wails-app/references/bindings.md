# Wails v3 Bindings & Service Lifecycle Reference

Reference for Wails v3 service contracts, data structures, and window lifecycle.

## Wails Service Registration

In `main.go`, `App` is registered as a Wails service:
```go
app = application.New(application.Options{
    Name:        "Obox App",
    Description: "POS Printer Proxy for Odoo",
    Icon:        appIcon,
    Services: []application.Service{
        application.NewService(oboxApp),
        application.NewService(notificationService),
    },
    Assets: application.AssetOptions{
        Handler: application.AssetFileServerFS(assets),
    },
})
```

- When the application boots, `App.ServiceStartup(ctx, options)` is executed.
- When shutting down, `App.ServiceShutdown()` is called, gracefully terminating background polling and closing printer endpoints.

## Core Data Structures

Shared between Go and TypeScript:

```go
type ServerInfo struct {
    IPAddress string `json:"ipAddress"`
    Port      int    `json:"port"`
    Running   bool   `json:"running"`
}

type SystemInfo struct {
    OS       string `json:"os"`
    Arch     string `json:"arch"`
    Hostname string `json:"hostname"`
}

type Settings struct {
    AutoStart              bool `json:"autoStart"`
    NetworkPrintingEnabled bool `json:"networkPrintingEnabled"`
    DebugMode              bool `json:"debugMode"`
}

type AppVariable struct {
    AppID    string              `json:"appId"`
    Server   ServerInfo          `json:"server"`
    System   SystemInfo          `json:"system"`
    Build    buildinfo.BuildInfo `json:"build"`
    Settings Settings            `json:"settings"`
}
```

Generated into TypeScript types in `frontend/bindings/obox-app/models.ts`.
