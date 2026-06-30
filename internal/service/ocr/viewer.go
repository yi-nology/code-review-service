package ocr

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/concurrency/async"
)

type ViewerManager struct {
	process  *exec.Cmd
	addr     string
	mu       sync.Mutex
	done     chan struct{}
	wg       sync.WaitGroup // PR-05：跟踪后台 cmd.Wait goroutine，避免 done 通道被覆盖时旧 goroutine 泄漏
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

func (vm *ViewerManager) Status() *domain.ViewerStatus {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process == nil {
		return &domain.ViewerStatus{Running: false, Message: "Viewer 未运行"}
	}

	select {
	case <-vm.done:
		vm.process = nil
		return &domain.ViewerStatus{Running: false, Message: "Viewer 已停止"}
	default:
	}

	return &domain.ViewerStatus{
		Running: true,
		Addr:    vm.addr,
		Message: fmt.Sprintf("Viewer 运行中 (%s)", vm.addr),
	}
}

func (vm *ViewerManager) ViewerStatus(ctx context.Context) (*domain.ViewerStatus, error) {
	return vm.Status(), nil
}

func (vm *ViewerManager) StartViewer(ctx context.Context, addr string) (*domain.ViewerStatus, error) {
	binary := vm.resolveBinary()
	args := []string{"viewer"}
	if addr != "" {
		args = append(args, "--addr", addr)
	} else {
		addr = "localhost:5483"
	}
	return vm.StartViewerWithCommand(ctx, addr, binary, args)
}

// StartViewerWithCommand 用指定 binary/args 启动 viewer 进程。
// 暴露出来用于测试（生产调用方仍用 StartViewer）。
func (vm *ViewerManager) StartViewerWithCommand(ctx context.Context, addr, binary string, args []string) (*domain.ViewerStatus, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process != nil {
		select {
		case <-vm.done:
			vm.process = nil
		default:
			return &domain.ViewerStatus{
				Running: true,
				Addr:    vm.addr,
				Message: "Viewer 已在运行",
			}, nil
		}
	}

	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Viewer 失败: %w", err)
	}

	// PR-05 修复：在覆盖 done 之前先 Wait 旧 goroutine 退出，
	// 避免旧 done 通道被遗弃导致 cmd/process 关联对象无法 GC。
	vm.wg.Wait()
	vm.process = cmd
	vm.addr = addr
	vm.done = make(chan struct{})

	vm.wg.Add(1)
	async.GoSafe(func() {
		defer vm.wg.Done()
		_ = cmd.Wait()
		close(vm.done)
	})

	return &domain.ViewerStatus{
		Running: true,
		Addr:    addr,
		Message: fmt.Sprintf("Viewer 已启动 (%s)", addr),
	}, nil
}

func (vm *ViewerManager) StopViewer(ctx context.Context) (*domain.ViewerStatus, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.process == nil {
		return &domain.ViewerStatus{Running: false, Message: "Viewer 未运行"}, nil
	}

	select {
	case <-vm.done:
		vm.process = nil
		return &domain.ViewerStatus{Running: false, Message: "Viewer 已停止"}, nil
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

	return &domain.ViewerStatus{
		Running: false,
		Addr:    addr,
		Message: "Viewer 已停止",
	}, nil
}

var _ domain.ViewerManager = (*ViewerManager)(nil)
