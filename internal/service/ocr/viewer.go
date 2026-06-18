package ocr

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/concurrency/async"
)

type ViewerManager struct {
	process  *exec.Cmd
	addr     string
	mu       sync.Mutex
	done     chan struct{}
	binary   string
	settings settingsSource
	resolver *ConfigResolver
}

func NewViewerManager(settings settingsSource, resolver *ConfigResolver) *ViewerManager {
	return &ViewerManager{settings: settings, resolver: resolver}
}

func (vm *ViewerManager) resolveBinary() string {
	if vm.binary != "" {
		return vm.binary
	}
	if vm.settings != nil {
		s, err := vm.settings.Get(context.Background())
		if err == nil && s != nil && s.Binary != "" {
			vm.binary = s.Binary
			return vm.binary
		}
	}
	if vm.resolver != nil {
		vm.binary = vm.resolver.fallback.Binary
	}
	if vm.binary == "" {
		vm.binary = "ocr"
	}
	return vm.binary
}

func (vm *ViewerManager) Status() *review.ViewerStatus {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process == nil {
		return &review.ViewerStatus{Running: false, Message: "Viewer 未运行"}
	}

	select {
	case <-vm.done:
		vm.process = nil
		return &review.ViewerStatus{Running: false, Message: "Viewer 已停止"}
	default:
	}

	return &review.ViewerStatus{
		Running: true,
		Addr:    vm.addr,
		Message: fmt.Sprintf("Viewer 运行中 (%s)", vm.addr),
	}
}

func (vm *ViewerManager) ViewerStatus(ctx context.Context) (*review.ViewerStatus, error) {
	return vm.Status(), nil
}

func (vm *ViewerManager) StartViewer(ctx context.Context, addr string) (*review.ViewerStatus, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process != nil {
		select {
		case <-vm.done:
			vm.process = nil
		default:
			return &review.ViewerStatus{
				Running: true,
				Addr:    vm.addr,
				Message: "Viewer 已在运行",
			}, nil
		}
	}

	binary := vm.resolveBinary()

	args := []string{"viewer"}
	if addr != "" {
		args = append(args, "--addr", addr)
	} else {
		addr = "localhost:5483"
	}

	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Viewer 失败: %w", err)
	}

	vm.process = cmd
	vm.addr = addr
	vm.done = make(chan struct{})

	async.GoSafe(func() {
		_ = cmd.Wait()
		close(vm.done)
	})

	return &review.ViewerStatus{
		Running: true,
		Addr:    addr,
		Message: fmt.Sprintf("Viewer 已启动 (%s)", addr),
	}, nil
}

func (vm *ViewerManager) StopViewer(ctx context.Context) (*review.ViewerStatus, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process == nil {
		return &review.ViewerStatus{Running: false, Message: "Viewer 未运行"}, nil
	}

	select {
	case <-vm.done:
		vm.process = nil
		return &review.ViewerStatus{Running: false, Message: "Viewer 已停止"}, nil
	default:
	}

	if err := syscall.Kill(-vm.process.Process.Pid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(-vm.process.Process.Pid, syscall.SIGKILL)
	}

	select {
	case <-vm.done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-vm.process.Process.Pid, syscall.SIGKILL)
	}

	addr := vm.addr
	vm.process = nil
	vm.addr = ""

	return &review.ViewerStatus{
		Running: false,
		Addr:    addr,
		Message: "Viewer 已停止",
	}, nil
}

var _ review.ViewerManager = (*ViewerManager)(nil)
