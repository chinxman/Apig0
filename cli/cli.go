package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"apig0/auth"
	"apig0/config"
	"apig0/middleware"
)

func Run(args []string) (bool, int) {
	if len(args) == 0 {
		return true, runStart(nil)
	}

	switch strings.TrimSpace(args[0]) {
	case "serve":
		return false, 0
	case "start":
		return true, runStart(args[1:])
	case "stop":
		return true, runStop(args[1:])
	case "restart":
		return true, runRestart(args[1:])
	case "help", "-h", "--help":
		printRootHelp()
		return true, 0
	case "logs":
		return true, runLogs(args[1:])
	case "monitor":
		return true, runMonitor(args[1:])
	case "status":
		return true, runStatus()
	case "setup":
		return true, runSetup(args[1:])
	case "users":
		return true, runUsers(args[1:])
	case "services":
		return true, runServices(args[1:])
	case "tokens":
		return true, runTokens(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printRootHelp()
		return true, 1
	}
}

func ShouldSilenceBootstrapLogs(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch strings.TrimSpace(args[0]) {
	case "serve", "start":
		return false
	default:
		return true
	}
}

func runStart(args []string) int {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logPath := fs.String("log-file", defaultLogPath(), "background log file")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if pid, ok := runningBackgroundPID(); ok {
		fmt.Fprintf(os.Stderr, "apig0 is already running in background (pid=%d)\n", pid)
		fmt.Fprintf(os.Stderr, "use `logs` to inspect runtime output\n")
		return 1
	}
	if err := ensurePortAvailable(config.GetRuntimeStatus().Port); err != nil {
		fmt.Fprintf(os.Stderr, "cannot start background server: %v\n", err)
		return 1
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve executable failed: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*logPath), 0700); err != nil && filepath.Dir(*logPath) != "." {
		fmt.Fprintf(os.Stderr, "prepare log path failed: %v\n", err)
		return 1
	}
	monitorPath := middleware.MonitorEventLogPath()
	if err := os.MkdirAll(filepath.Dir(monitorPath), 0700); err != nil && filepath.Dir(monitorPath) != "." {
		fmt.Fprintf(os.Stderr, "prepare monitor path failed: %v\n", err)
		return 1
	}

	logFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open log file failed: %v\n", err)
		return 1
	}
	defer logFile.Close()
	monitorFile, err := os.OpenFile(monitorPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open monitor file failed: %v\n", err)
		return 1
	}
	_ = monitorFile.Close()

	cmd := exec.Command(exe, "serve")
	cmd.Dir, _ = os.Getwd()
	cmd.Env = append(os.Environ(), "APIG0_BACKGROUND=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "background start failed: %v\n", err)
		return 1
	}
	if err := writePIDFile(cmd.Process.Pid); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write pid file: %v\n", err)
	}

	scheme := "http"
	if config.LoadTLSConfig().Mode != config.TLSOff {
		scheme = "https"
	}
	status := config.GetRuntimeStatus()
	host := advertisedHost()

	fmt.Printf("apig0 started in background (pid=%d)\n", cmd.Process.Pid)
	fmt.Printf("url: %s://%s:%s/\n", scheme, host, status.Port)
	fmt.Printf("setup: %s\n", status.SetupMode)
	fmt.Printf("log_file: %s\n", *logPath)
	fmt.Println("use `go run main.go logs` to inspect runtime output")
	return 0
}

func runLogs(args []string) int {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	follow := fs.Bool("f", false, "follow log output")
	lines := fs.Int("n", 80, "number of trailing lines to show")
	logPath := fs.String("log-file", defaultLogPath(), "background log file")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	raw, err := os.ReadFile(*logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read log failed: %v\n", err)
		return 1
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	allLines := strings.Split(content, "\n")
	if len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}
	start := 0
	if *lines > 0 && len(allLines) > *lines {
		start = len(allLines) - *lines
	}
	for _, line := range allLines[start:] {
		fmt.Println(line)
	}
	if !*follow {
		return 0
	}

	file, err := os.Open(*logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open log failed: %v\n", err)
		return 1
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		fmt.Fprintf(os.Stderr, "seek log failed: %v\n", err)
		return 1
	}

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err == nil {
			fmt.Print(line)
			continue
		}
		if errors.Is(err, io.EOF) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		fmt.Fprintf(os.Stderr, "follow log failed: %v\n", err)
		return 1
	}
}

func runStop(args []string) int {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	force := fs.Bool("force", false, "force kill if graceful stop times out")
	timeout := fs.Duration("timeout", 5*time.Second, "graceful stop wait time")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	pid, ok := runningBackgroundPID()
	if !ok {
		fmt.Fprintln(os.Stderr, "no managed background apig0 process is running")
		return 1
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(defaultPIDPath())
		fmt.Fprintf(os.Stderr, "could not find process %d\n", pid)
		return 1
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		_ = os.Remove(defaultPIDPath())
		fmt.Fprintf(os.Stderr, "stop failed: %v\n", err)
		return 1
	}

	deadline := time.Now().Add(*timeout)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			_ = os.Remove(defaultPIDPath())
			fmt.Printf("apig0 stopped (pid=%d)\n", pid)
			return 0
		}
		time.Sleep(200 * time.Millisecond)
	}

	if *force {
		if err := process.Signal(syscall.SIGKILL); err != nil {
			fmt.Fprintf(os.Stderr, "force stop failed: %v\n", err)
			return 1
		}
		for i := 0; i < 10; i++ {
			if !pidAlive(pid) {
				_ = os.Remove(defaultPIDPath())
				fmt.Printf("apig0 force-stopped (pid=%d)\n", pid)
				return 0
			}
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Fprintf(os.Stderr, "process %d did not exit after SIGKILL\n", pid)
		return 1
	}

	fmt.Fprintf(os.Stderr, "process %d did not exit within %s\n", pid, timeout.String())
	fmt.Fprintln(os.Stderr, "retry with `stop --force` if needed")
	return 1
}

func runRestart(args []string) int {
	if _, ok := runningBackgroundPID(); ok {
		if code := runStop(nil); code != 0 {
			return code
		}
	}
	return runStart(args)
}

func runMonitor(args []string) int {
	fs := flag.NewFlagSet("monitor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	follow := fs.Bool("f", false, "follow request events")
	lines := fs.Int("n", 40, "number of recent request events to show")
	service := fs.String("service", "", "filter by service name")
	errorsOnly := fs.Bool("errors", false, "show only HTTP 4xx/5xx events")
	monitorPath := fs.String("file", middleware.MonitorEventLogPath(), "structured monitor file")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	events, err := readMonitorEvents(*monitorPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "no monitor stream found; start the gateway first")
			return 1
		}
		fmt.Fprintf(os.Stderr, "read monitor failed: %v\n", err)
		return 1
	}

	filter := func(evt middleware.RequestEvent) bool {
		if strings.TrimSpace(*service) != "" && evt.Service != strings.TrimSpace(*service) {
			return false
		}
		if *errorsOnly && evt.Status < 400 {
			return false
		}
		return true
	}

	filtered := make([]middleware.RequestEvent, 0, len(events))
	for _, evt := range events {
		if filter(evt) {
			filtered = append(filtered, evt)
		}
	}
	start := 0
	if *lines > 0 && len(filtered) > *lines {
		start = len(filtered) - *lines
	}
	for _, evt := range filtered[start:] {
		fmt.Println(formatMonitorEvent(evt))
	}
	if !*follow {
		return 0
	}

	file, err := os.Open(*monitorPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open monitor failed: %v\n", err)
		return 1
	}
	defer file.Close()
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		fmt.Fprintf(os.Stderr, "seek monitor failed: %v\n", err)
		return 1
	}
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err == nil {
			var evt middleware.RequestEvent
			if json.Unmarshal([]byte(strings.TrimSpace(line)), &evt) == nil && filter(evt) {
				fmt.Println(formatMonitorEvent(evt))
			}
			continue
		}
		if errors.Is(err, io.EOF) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		fmt.Fprintf(os.Stderr, "follow monitor failed: %v\n", err)
		return 1
	}
}

func runStatus() int {
	status := config.GetRuntimeStatus()
	w := newWriter()
	defer w.Flush()

	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "setup_required\t%t\n", status.SetupRequired)
	fmt.Fprintf(w, "setup_mode\t%s\n", status.SetupMode)
	fmt.Fprintf(w, "persistent_configured\t%t\n", status.PersistentConfigured)
	fmt.Fprintf(w, "port\t%s\n", status.Port)
	fmt.Fprintf(w, "admins\t%d\n", status.AdminCount)
	fmt.Fprintf(w, "users_backend\t%s\n", status.UsersBackend)
	fmt.Fprintf(w, "users_path\t%s\n", status.UsersPath)
	fmt.Fprintf(w, "totp_backend\t%s\n", status.SecretsBackend)
	if status.SecretsPath != "" {
		fmt.Fprintf(w, "totp_path\t%s\n", status.SecretsPath)
	}
	fmt.Fprintf(w, "service_secret_mode\t%s\n", status.ServiceSecrets.Mode)
	if status.ServiceSecrets.FilePath != "" {
		fmt.Fprintf(w, "service_secret_path\t%s\n", status.ServiceSecrets.FilePath)
	}
	fmt.Fprintf(w, "service_count\t%d\n", status.ServiceCount)
	fmt.Fprintf(w, "api_token_count\t%d\n", status.APITokenCount)
	fmt.Fprintf(w, "policy_user_count\t%d\n", status.PolicyUserCount)
	fmt.Fprintf(w, "audit_log_path\t%s\n", status.AuditLogPath)
	return 0
}

func runSetup(args []string) int {
	if len(args) == 0 {
		printSetupHelp()
		return 1
	}
	switch args[0] {
	case "status":
		return runStatus()
	case "reset":
		fs := flag.NewFlagSet("setup reset", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		force := fs.Bool("force", false, "required to wipe setup state")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if !*force {
			fmt.Fprintln(os.Stderr, "setup reset requires --force")
			return 1
		}
		if err := config.ResetSetupState(); err != nil {
			fmt.Fprintf(os.Stderr, "reset failed: %v\n", err)
			return 1
		}
		if err := config.ReloadRuntime(nil, ""); err != nil {
			fmt.Fprintf(os.Stderr, "runtime reload failed after reset: %v\n", err)
			return 1
		}
		fmt.Println("setup state reset")
		return 0
	case "bootstrap-admin":
		fs := flag.NewFlagSet("setup bootstrap-admin", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "admin username")
		password := fs.String("password", "", "admin password")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if config.GetRuntimeStatus().SetupRequired {
			fmt.Fprintln(os.Stderr, "complete initial setup first")
			return 1
		}
		if config.GetRuntimeStatus().HasAdmin {
			fmt.Fprintln(os.Stderr, "bootstrap disabled: admin already exists")
			return 1
		}
		if strings.TrimSpace(*username) == "" || *password == "" {
			fmt.Fprintln(os.Stderr, "--username and --password are required")
			return 1
		}
		_, otpauth, err := auth.ProvisionUser(*username, *password, "admin", nil, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
			return 1
		}
		fmt.Printf("admin created: %s\n", strings.TrimSpace(*username))
		fmt.Printf("otpauth: %s\n", otpauth)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown setup command: %s\n\n", args[0])
		printSetupHelp()
		return 1
	}
}

func runUsers(args []string) int {
	if len(args) == 0 {
		printUsersHelp()
		return 1
	}
	switch args[0] {
	case "list":
		users := config.GetUserStore().List()
		w := newWriter()
		defer w.Flush()
		fmt.Fprintln(w, "USERNAME\tROLE\tSERVICE_ACCESS\tALLOWED_SERVICES\tCREATED")
		for _, user := range users {
			scope := "all"
			allowed := "-"
			if user.Role != "admin" && user.ServiceAccessConfigured {
				scope = "restricted"
				if len(user.AllowedServices) > 0 {
					allowed = strings.Join(user.AllowedServices, ",")
				} else {
					allowed = "(none)"
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", user.Username, user.Role, scope, allowed, user.CreatedAt.Format(time.RFC3339))
		}
		return 0
	case "add":
		fs := flag.NewFlagSet("users add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username")
		password := fs.String("password", "", "password")
		role := fs.String("role", "user", "user or admin")
		services := fs.String("services", "", "comma-separated allowed services for non-admin users")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		roleValue := strings.ToLower(strings.TrimSpace(*role))
		if roleValue != "user" && roleValue != "admin" {
			fmt.Fprintln(os.Stderr, "--role must be user or admin")
			return 1
		}
		if strings.TrimSpace(*username) == "" || *password == "" {
			fmt.Fprintln(os.Stderr, "--username and --password are required")
			return 1
		}
		allowed := parseCSV(*services)
		restrictServices := roleValue != "admin"
		_, otpauth, err := auth.ProvisionUser(strings.TrimSpace(*username), *password, roleValue, allowed, restrictServices)
		if err != nil {
			fmt.Fprintf(os.Stderr, "user creation failed: %v\n", err)
			return 1
		}
		fmt.Printf("user created: %s\n", strings.TrimSpace(*username))
		fmt.Printf("role: %s\n", roleValue)
		if restrictServices {
			fmt.Printf("allowed_services: %s\n", strings.Join(config.NormalizeAllowedServices(allowed), ","))
		}
		fmt.Printf("otpauth: %s\n", otpauth)
		return 0
	case "delete":
		fs := flag.NewFlagSet("users delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "username")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if strings.TrimSpace(*username) == "" {
			fmt.Fprintln(os.Stderr, "--username is required")
			return 1
		}
		if err := config.GetUserStore().Delete(strings.TrimSpace(*username)); err != nil {
			fmt.Fprintf(os.Stderr, "delete failed: %v\n", err)
			return 1
		}
		config.DeleteUserSecret(strings.TrimSpace(*username))
		fmt.Printf("user deleted: %s\n", strings.TrimSpace(*username))
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown users command: %s\n\n", args[0])
		printUsersHelp()
		return 1
	}
}

func runServices(args []string) int {
	if len(args) == 0 {
		printServicesHelp()
		return 1
	}
	switch args[0] {
	case "list":
		services := config.GetServiceCatalog()
		w := newWriter()
		defer w.Flush()
		fmt.Fprintln(w, "NAME\tURL\tAUTH\tSECRET\tTIMEOUT_MS\tRETRIES\tENABLED")
		for _, svc := range services {
			fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%d\t%d\t%t\n", svc.Name, svc.BaseURL, svc.AuthType, svc.HasSecret, svc.TimeoutMS, svc.RetryCount, svc.Enabled)
		}
		return 0
	case "add":
		fs := flag.NewFlagSet("services add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "service name")
		baseURL := fs.String("url", "", "base upstream URL")
		authType := fs.String("auth-type", "none", "none, bearer, x-api-key, custom-header, basic")
		headerName := fs.String("header", "", "header name for x-api-key or custom-header")
		basicUsername := fs.String("basic-username", "", "username for basic auth")
		timeoutMS := fs.Int("timeout-ms", 10000, "request timeout in milliseconds")
		retryCount := fs.Int("retry-count", 0, "retry count (0-3)")
		secret := fs.String("secret", "", "stored upstream credential")
		enabled := fs.Bool("enabled", true, "whether the service is enabled")
		secretNotes := fs.String("secret-notes", "", "metadata note for stored secret")
		secretExpires := fs.String("secret-expires-at", "", "RFC3339 expiry for stored secret")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if strings.TrimSpace(*name) == "" || strings.TrimSpace(*baseURL) == "" {
			fmt.Fprintln(os.Stderr, "--name and --url are required")
			return 1
		}
		if _, exists := config.GetServiceConfig(*name); exists {
			fmt.Fprintln(os.Stderr, "service already exists")
			return 1
		}
		cfg := config.ServiceConfig{
			Name:          strings.TrimSpace(*name),
			BaseURL:       strings.TrimSpace(*baseURL),
			AuthType:      config.ServiceAuthType(strings.TrimSpace(*authType)),
			HeaderName:    strings.TrimSpace(*headerName),
			BasicUsername: strings.TrimSpace(*basicUsername),
			TimeoutMS:     *timeoutMS,
			RetryCount:    *retryCount,
			Enabled:       *enabled,
			HasSecret:     strings.TrimSpace(*secret) != "",
		}
		if strings.TrimSpace(*secret) != "" && config.ServiceSecretStatus().Locked {
			fmt.Fprintln(os.Stderr, "service secret storage is locked; unlock it before changing saved keys")
			return 1
		}
		if err := config.UpsertService(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "service create failed: %v\n", err)
			return 1
		}
		if strings.TrimSpace(*secret) != "" {
			if err := config.SetServiceSecret(cfg.Name, strings.TrimSpace(*secret)); err != nil {
				_ = config.DeleteService(cfg.Name)
				fmt.Fprintf(os.Stderr, "service secret save failed: %v\n", err)
				return 1
			}
			if expiresAt, err := parseOptionalTimestamp(*secretExpires); err == nil {
				_ = config.NoteServiceSecretRotated(cfg.Name, expiresAt, *secretNotes)
			}
		}
		fmt.Printf("service created: %s\n", config.NormalizeAllowedServiceName(*name))
		return 0
	case "delete":
		fs := flag.NewFlagSet("services delete", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "service name")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if strings.TrimSpace(*name) == "" {
			fmt.Fprintln(os.Stderr, "--name is required")
			return 1
		}
		serviceName := config.NormalizeAllowedServiceName(*name)
		existing, ok := config.GetServiceConfig(serviceName)
		if !ok {
			fmt.Fprintln(os.Stderr, "service not found")
			return 1
		}
		if existing.HasSecret && config.ServiceSecretStatus().Locked {
			fmt.Fprintln(os.Stderr, "service secret storage is locked; unlock it before deleting a saved key")
			return 1
		}
		if existing.HasSecret {
			if err := config.DeleteServiceSecret(serviceName); err != nil && !errors.Is(err, config.ErrServiceSecretsLocked) {
				fmt.Fprintf(os.Stderr, "service secret delete failed: %v\n", err)
				return 1
			}
			_ = config.DeleteServiceSecretMetadata(serviceName)
		}
		if err := config.DeleteService(serviceName); err != nil {
			fmt.Fprintf(os.Stderr, "service delete failed: %v\n", err)
			return 1
		}
		if store := config.GetUserStore(); store != nil {
			if err := store.RemoveAllowedServiceEverywhere(serviceName); err != nil {
				fmt.Fprintf(os.Stderr, "service removed but user access cleanup failed: %v\n", err)
				return 1
			}
		}
		fmt.Printf("service deleted: %s\n", serviceName)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown services command: %s\n\n", args[0])
		printServicesHelp()
		return 1
	}
}

func runTokens(args []string) int {
	if len(args) == 0 {
		printTokensHelp()
		return 1
	}
	switch args[0] {
	case "list":
		tokens := config.ListAPITokens()
		w := newWriter()
		defer w.Flush()
		fmt.Fprintln(w, "ID\tNAME\tUSER\tPREFIX\tSERVICES\tEXPIRES_AT\tREVOKED")
		for _, token := range tokens {
			expiresAt := "-"
			if !token.ExpiresAt.IsZero() {
				expiresAt = token.ExpiresAt.Format(time.RFC3339)
			}
			services := "-"
			if len(token.AllowedServices) > 0 {
				services = strings.Join(token.AllowedServices, ",")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%t\n", token.ID, token.Name, token.User, token.TokenPrefix, services, expiresAt, !token.RevokedAt.IsZero())
		}
		return 0
	case "create":
		fs := flag.NewFlagSet("tokens create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "token name")
		user := fs.String("user", "", "user that owns the token")
		services := fs.String("services", "", "comma-separated allowed services")
		expiresAt := fs.String("expires-at", "", "RFC3339 expiry")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if strings.TrimSpace(*user) == "" {
			fmt.Fprintln(os.Stderr, "--user is required")
			return 1
		}
		if !config.GetUserStore().Exists(strings.TrimSpace(*user)) {
			fmt.Fprintln(os.Stderr, "user not found")
			return 1
		}
		expiry, err := parseOptionalTimestamp(*expiresAt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid --expires-at; use RFC3339")
			return 1
		}
		raw, token, err := config.CreateAPIToken(config.APITokenCreateParams{
			Name:            *name,
			User:            strings.TrimSpace(*user),
			AllowedServices: parseCSV(*services),
			ExpiresAt:       expiry,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "token create failed: %v\n", err)
			return 1
		}
		fmt.Printf("token id: %s\n", token.ID)
		fmt.Printf("token: %s\n", raw)
		fmt.Printf("prefix: %s\n", token.TokenPrefix)
		return 0
	case "revoke":
		fs := flag.NewFlagSet("tokens revoke", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "token id")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if strings.TrimSpace(*id) == "" {
			fmt.Fprintln(os.Stderr, "--id is required")
			return 1
		}
		if err := config.RevokeAPIToken(strings.TrimSpace(*id)); err != nil {
			fmt.Fprintf(os.Stderr, "token revoke failed: %v\n", err)
			return 1
		}
		fmt.Printf("token revoked: %s\n", strings.TrimSpace(*id))
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown tokens command: %s\n\n", args[0])
		printTokensHelp()
		return 1
	}
}

func parseCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func parseOptionalTimestamp(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func newWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
}

func defaultLogPath() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_LOG_PATH")); path != "" {
		return path
	}
	return filepath.Join(os.TempDir(), "apig0-runtime.log")
}

func readMonitorEvents(path string) ([]middleware.RequestEvent, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	out := make([]middleware.RequestEvent, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt middleware.RequestEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		out = append(out, evt)
	}
	return out, nil
}

func formatMonitorEvent(evt middleware.RequestEvent) string {
	ts := time.UnixMilli(evt.Timestamp).Format("15:04:05")
	service := evt.Service
	if service == "" {
		service = "-"
	}
	user := evt.User
	if user == "" {
		user = "-"
	}
	return fmt.Sprintf("%s  %-6s %-3d %7.2fms  %-14s  %-12s  %s", ts, evt.Method, evt.Status, evt.LatencyMs, service, user, evt.Path)
}

func defaultPIDPath() string {
	if path := strings.TrimSpace(os.Getenv("APIG0_PID_PATH")); path != "" {
		return path
	}
	return filepath.Join(os.TempDir(), "apig0-runtime.pid")
}

func writePIDFile(pid int) error {
	return os.WriteFile(defaultPIDPath(), []byte(fmt.Sprintf("%d\n", pid)), 0600)
}

func runningBackgroundPID() (int, bool) {
	raw, err := os.ReadFile(defaultPIDPath())
	if err != nil {
		return 0, false
	}
	pidText := strings.TrimSpace(string(raw))
	if pidText == "" {
		_ = os.Remove(defaultPIDPath())
		return 0, false
	}
	var pid int
	if _, err := fmt.Sscanf(pidText, "%d", &pid); err != nil || pid <= 0 {
		_ = os.Remove(defaultPIDPath())
		return 0, false
	}
	if !pidAlive(pid) {
		_ = os.Remove(defaultPIDPath())
		return 0, false
	}
	return pid, true
}

func pidAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func ensurePortAvailable(port string) error {
	port = strings.TrimSpace(port)
	if port == "" {
		port = "8989"
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	_ = ln.Close()
	return nil
}

func advertisedHost() string {
	if host := strings.TrimSpace(os.Getenv("APIG0_ADVERTISE_HOST")); host != "" {
		return host
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return "127.0.0.1"
}

func printRootHelp() {
	fmt.Println("Usage: apig0 <command> [subcommand] [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start                 Start the web gateway server in background")
	fmt.Println("  stop                  Stop the managed background server")
	fmt.Println("  restart               Restart the managed background server")
	fmt.Println("  serve                 Start the web gateway server in foreground")
	fmt.Println("  logs                  Show background server logs")
	fmt.Println("  monitor               Show live structured request activity")
	fmt.Println("  status                Show runtime and storage status")
	fmt.Println("  setup status          Show setup/runtime status")
	fmt.Println("  setup reset           Reset setup and persistent state (requires --force)")
	fmt.Println("  setup bootstrap-admin Create an admin after setup when no admin exists")
	fmt.Println("  users list            List users")
	fmt.Println("  users add             Create a user or admin")
	fmt.Println("  users delete          Delete a user")
	fmt.Println("  services list         List services")
	fmt.Println("  services add          Create a service")
	fmt.Println("  services delete       Delete a service")
	fmt.Println("  tokens list           List API tokens")
	fmt.Println("  tokens create         Create an API token")
	fmt.Println("  tokens revoke         Revoke an API token")
	fmt.Println()
	fmt.Println("Bare `apig0` defaults to `apig0 start`.")
}

func printSetupHelp() {
	fmt.Println("Usage: apig0 setup <status|reset|bootstrap-admin> [flags]")
}

func printUsersHelp() {
	fmt.Println("Usage: apig0 users <list|add|delete> [flags]")
}

func printServicesHelp() {
	fmt.Println("Usage: apig0 services <list|add|delete> [flags]")
}

func printTokensHelp() {
	fmt.Println("Usage: apig0 tokens <list|create|revoke> [flags]")
}
