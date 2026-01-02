package hierarchy

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewWorkerPool(t *testing.T) {
	pool := NewWorkerPool("pool-1", "supervisor-1", DefaultPoolConfig())

	if pool.ID() != "pool-1" {
		t.Errorf("expected pool ID 'pool-1', got %q", pool.ID())
	}

	if pool.SupervisorID() != "supervisor-1" {
		t.Errorf("expected supervisor ID 'supervisor-1', got %q", pool.SupervisorID())
	}

	if pool.Size() != 0 {
		t.Errorf("expected empty pool, got size %d", pool.Size())
	}

	config := pool.Config()
	if config.MaxSize != 10 {
		t.Errorf("expected default max size 10, got %d", config.MaxSize)
	}
}

func TestWorkerPoolAddWorker(t *testing.T) {
	config := DefaultPoolConfig()
	config.MaxSize = 3
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	// Add first worker
	worker1 := &HierarchyNode{
		AgentID: "worker-1",
		Role:    RoleWorker,
		Status:  "active",
	}

	err := pool.AddWorker(worker1)
	if err != nil {
		t.Fatalf("failed to add worker: %v", err)
	}

	if pool.Size() != 1 {
		t.Errorf("expected pool size 1, got %d", pool.Size())
	}

	// Add second worker
	worker2 := &HierarchyNode{
		AgentID: "worker-2",
		Role:    RoleWorker,
		Status:  "active",
	}

	err = pool.AddWorker(worker2)
	if err != nil {
		t.Fatalf("failed to add second worker: %v", err)
	}

	if pool.Size() != 2 {
		t.Errorf("expected pool size 2, got %d", pool.Size())
	}

	// Verify workers can be retrieved
	w1, exists := pool.GetWorker("worker-1")
	if !exists || w1.AgentID != "worker-1" {
		t.Error("failed to retrieve worker-1")
	}

	w2, exists := pool.GetWorker("worker-2")
	if !exists || w2.AgentID != "worker-2" {
		t.Error("failed to retrieve worker-2")
	}
}

func TestWorkerPoolMaxSizeLimit(t *testing.T) {
	config := DefaultPoolConfig()
	config.MaxSize = 2
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	// Add two workers (at capacity)
	for i := 1; i <= 2; i++ {
		worker := &HierarchyNode{
			AgentID: "worker-" + string(rune('0'+i)),
			Role:    RoleWorker,
			Status:  "active",
		}
		if err := pool.AddWorker(worker); err != nil {
			t.Fatalf("failed to add worker %d: %v", i, err)
		}
	}

	if !pool.IsFull() {
		t.Error("expected pool to be full")
	}

	if pool.AvailableSlots() != 0 {
		t.Errorf("expected 0 available slots, got %d", pool.AvailableSlots())
	}

	// Try to add third worker (should fail)
	worker3 := &HierarchyNode{
		AgentID: "worker-3",
		Role:    RoleWorker,
		Status:  "active",
	}

	err := pool.AddWorker(worker3)
	if err == nil {
		t.Error("expected error when adding worker to full pool")
	}
}

func TestWorkerPoolRemoveWorker(t *testing.T) {
	pool := NewWorkerPool("pool-1", "supervisor-1", DefaultPoolConfig())

	worker := &HierarchyNode{
		AgentID: "worker-1",
		Role:    RoleWorker,
		Status:  "active",
	}

	_ = pool.AddWorker(worker)

	if pool.Size() != 1 {
		t.Fatalf("expected pool size 1, got %d", pool.Size())
	}

	err := pool.RemoveWorker("worker-1")
	if err != nil {
		t.Fatalf("failed to remove worker: %v", err)
	}

	if pool.Size() != 0 {
		t.Errorf("expected pool size 0 after removal, got %d", pool.Size())
	}

	// Try to remove non-existent worker
	err = pool.RemoveWorker("worker-1")
	if err == nil {
		t.Error("expected error when removing non-existent worker")
	}
}

func TestWorkerPoolDuplicateWorker(t *testing.T) {
	pool := NewWorkerPool("pool-1", "supervisor-1", DefaultPoolConfig())

	worker := &HierarchyNode{
		AgentID: "worker-1",
		Role:    RoleWorker,
		Status:  "active",
	}

	_ = pool.AddWorker(worker)

	// Try to add same worker again
	err := pool.AddWorker(worker)
	if err == nil {
		t.Error("expected error when adding duplicate worker")
	}
}

func TestWorkerPoolHealthTracking(t *testing.T) {
	pool := NewWorkerPool("pool-1", "supervisor-1", DefaultPoolConfig())

	worker := &HierarchyNode{
		AgentID: "worker-1",
		Role:    RoleWorker,
		Status:  "active",
	}

	_ = pool.AddWorker(worker)

	// Check initial health status
	health, exists := pool.GetWorkerHealth("worker-1")
	if !exists {
		t.Fatal("health record not found for worker")
	}

	if health.Status != HealthStatusUnknown {
		t.Errorf("expected initial health status 'unknown', got %q", health.Status)
	}

	// Mark worker healthy
	err := pool.MarkWorkerHealthy("worker-1")
	if err != nil {
		t.Fatalf("failed to mark worker healthy: %v", err)
	}

	health, _ = pool.GetWorkerHealth("worker-1")
	if health.Status != HealthStatusHealthy {
		t.Errorf("expected health status 'healthy', got %q", health.Status)
	}

	// Mark worker unhealthy (without auto-remove)
	config := DefaultPoolConfig()
	config.AutoRemoveUnhealthy = false
	pool2 := NewWorkerPool("pool-2", "supervisor-1", config)
	_ = pool2.AddWorker(&HierarchyNode{AgentID: "worker-2", Role: RoleWorker, Status: "active"})

	err = pool2.MarkWorkerUnhealthy("worker-2", "test failure")
	if err != nil {
		t.Fatalf("failed to mark worker unhealthy: %v", err)
	}

	health, _ = pool2.GetWorkerHealth("worker-2")
	if health.Status != HealthStatusUnhealthy {
		t.Errorf("expected health status 'unhealthy', got %q", health.Status)
	}

	if health.ErrorMessage != "test failure" {
		t.Errorf("expected error message 'test failure', got %q", health.ErrorMessage)
	}
}

func TestWorkerPoolAutoRemoveUnhealthy(t *testing.T) {
	config := DefaultPoolConfig()
	config.AutoRemoveUnhealthy = true
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	worker := &HierarchyNode{
		AgentID: "worker-1",
		Role:    RoleWorker,
		Status:  "active",
	}

	_ = pool.AddWorker(worker)

	if pool.Size() != 1 {
		t.Fatalf("expected pool size 1, got %d", pool.Size())
	}

	// Mark worker unhealthy - should auto-remove
	_ = pool.MarkWorkerUnhealthy("worker-1", "test failure")

	if pool.Size() != 0 {
		t.Errorf("expected pool size 0 after auto-remove, got %d", pool.Size())
	}
}

func TestWorkerPoolHealthChecker(t *testing.T) {
	config := DefaultPoolConfig()
	config.UnhealthyThreshold = 2
	config.DegradedThreshold = 1
	config.AutoRemoveUnhealthy = false
	config.HealthCheckInterval = 10 * time.Millisecond
	config.HealthCheckTimeout = 50 * time.Millisecond
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	// Add workers
	for i := 1; i <= 3; i++ {
		worker := &HierarchyNode{
			AgentID: "worker-" + string(rune('0'+i)),
			Role:    RoleWorker,
			Status:  "active",
		}
		_ = pool.AddWorker(worker)
	}

	// Create health checker that fails for worker-2
	healthyWorkers := map[string]bool{
		"worker-1": true,
		"worker-2": false, // Will be marked unhealthy
		"worker-3": true,
	}

	checker := func(ctx context.Context, workerID string) (bool, int64, error) {
		healthy := healthyWorkers[workerID]
		return healthy, 10, nil
	}

	pool.SetHealthChecker(checker)

	// Run health checks
	ctx := context.Background()
	pool.RunHealthChecks(ctx)

	// Check health statuses
	h1, _ := pool.GetWorkerHealth("worker-1")
	if h1.Status != HealthStatusHealthy {
		t.Errorf("expected worker-1 healthy, got %s", h1.Status)
	}

	h2, _ := pool.GetWorkerHealth("worker-2")
	if h2.Status != HealthStatusDegraded {
		t.Errorf("expected worker-2 degraded after 1 failure, got %s", h2.Status)
	}

	// Run health checks again - worker-2 should now be unhealthy
	pool.RunHealthChecks(ctx)

	h2, _ = pool.GetWorkerHealth("worker-2")
	if h2.Status != HealthStatusUnhealthy {
		t.Errorf("expected worker-2 unhealthy after 2 failures, got %s", h2.Status)
	}

	if h2.ConsecutiveFailures != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", h2.ConsecutiveFailures)
	}
}

func TestWorkerPoolListByHealth(t *testing.T) {
	config := DefaultPoolConfig()
	config.AutoRemoveUnhealthy = false
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	// Add workers
	for i := 1; i <= 3; i++ {
		worker := &HierarchyNode{
			AgentID: "worker-" + string(rune('0'+i)),
			Role:    RoleWorker,
			Status:  "active",
		}
		_ = pool.AddWorker(worker)
	}

	// Set different health statuses
	_ = pool.MarkWorkerHealthy("worker-1")
	_ = pool.MarkWorkerHealthy("worker-2")
	_ = pool.MarkWorkerUnhealthy("worker-3", "test")

	healthy := pool.ListHealthyWorkers()
	if len(healthy) != 2 {
		t.Errorf("expected 2 healthy workers, got %d", len(healthy))
	}

	unhealthy := pool.ListUnhealthyWorkers()
	if len(unhealthy) != 1 {
		t.Errorf("expected 1 unhealthy worker, got %d", len(unhealthy))
	}
}

func TestWorkerPoolStats(t *testing.T) {
	config := DefaultPoolConfig()
	config.MaxSize = 5
	config.MinSize = 1
	config.AutoRemoveUnhealthy = false
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	// Add workers
	for i := 1; i <= 3; i++ {
		worker := &HierarchyNode{
			AgentID: "worker-" + string(rune('0'+i)),
			Role:    RoleWorker,
			Status:  "active",
		}
		_ = pool.AddWorker(worker)
	}

	_ = pool.MarkWorkerHealthy("worker-1")
	_ = pool.MarkWorkerUnhealthy("worker-3", "test")

	stats := pool.Stats()

	if stats.TotalWorkers != 3 {
		t.Errorf("expected 3 total workers, got %d", stats.TotalWorkers)
	}

	if stats.HealthyWorkers != 1 {
		t.Errorf("expected 1 healthy worker, got %d", stats.HealthyWorkers)
	}

	if stats.UnhealthyWorkers != 1 {
		t.Errorf("expected 1 unhealthy worker, got %d", stats.UnhealthyWorkers)
	}

	if stats.UnknownWorkers != 1 {
		t.Errorf("expected 1 unknown worker, got %d", stats.UnknownWorkers)
	}

	if stats.MaxSize != 5 {
		t.Errorf("expected max size 5, got %d", stats.MaxSize)
	}

	if stats.AvailableSlots != 2 {
		t.Errorf("expected 2 available slots, got %d", stats.AvailableSlots)
	}
}

func TestWorkerPoolReplaceWorker(t *testing.T) {
	pool := NewWorkerPool("pool-1", "supervisor-1", DefaultPoolConfig())

	oldWorker := &HierarchyNode{
		AgentID: "old-worker",
		Role:    RoleWorker,
		Status:  "active",
	}

	_ = pool.AddWorker(oldWorker)
	_ = pool.MarkWorkerHealthy("old-worker")

	newWorker := &HierarchyNode{
		AgentID: "new-worker",
		Role:    RoleWorker,
		Status:  "active",
	}

	err := pool.ReplaceWorker("old-worker", newWorker)
	if err != nil {
		t.Fatalf("failed to replace worker: %v", err)
	}

	// Verify old worker is gone
	_, exists := pool.GetWorker("old-worker")
	if exists {
		t.Error("old worker should not exist after replacement")
	}

	// Verify new worker exists
	w, exists := pool.GetWorker("new-worker")
	if !exists || w.AgentID != "new-worker" {
		t.Error("new worker should exist after replacement")
	}

	// Verify pool size is unchanged
	if pool.Size() != 1 {
		t.Errorf("expected pool size 1 after replacement, got %d", pool.Size())
	}

	// Verify new worker has unknown health (not inherited)
	health, _ := pool.GetWorkerHealth("new-worker")
	if health.Status != HealthStatusUnknown {
		t.Errorf("expected new worker to have unknown health, got %s", health.Status)
	}
}

func TestWorkerPoolEventHandler(t *testing.T) {
	config := DefaultPoolConfig()
	config.AutoRemoveUnhealthy = false
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	var events []*WorkerPoolEvent
	var mu sync.Mutex

	pool.SetEventHandler(func(event *WorkerPoolEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	// Add a worker (should trigger event)
	worker := &HierarchyNode{
		AgentID: "worker-1",
		Role:    RoleWorker,
		Status:  "active",
	}
	_ = pool.AddWorker(worker)

	// Wait for async event
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventWorkerAdded {
		t.Errorf("expected EventWorkerAdded, got %s", events[0].Type)
	}
	mu.Unlock()

	// Mark healthy then unhealthy (should trigger health change events)
	_ = pool.MarkWorkerHealthy("worker-1")
	time.Sleep(50 * time.Millisecond)
	_ = pool.MarkWorkerUnhealthy("worker-1", "test")
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(events))
	}
	mu.Unlock()
}

func TestWorkerPoolHealthMonitoring(t *testing.T) {
	config := DefaultPoolConfig()
	config.HealthCheckInterval = 20 * time.Millisecond
	config.HealthCheckTimeout = 10 * time.Millisecond
	config.AutoRemoveUnhealthy = false
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	// Add workers
	for i := 1; i <= 2; i++ {
		worker := &HierarchyNode{
			AgentID: "worker-" + string(rune('0'+i)),
			Role:    RoleWorker,
			Status:  "active",
		}
		_ = pool.AddWorker(worker)
	}

	var checkCount int32

	pool.SetHealthChecker(func(ctx context.Context, workerID string) (bool, int64, error) {
		atomic.AddInt32(&checkCount, 1)
		return true, 5, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pool.StartHealthMonitoring(ctx)
	if err != nil {
		t.Fatalf("failed to start health monitoring: %v", err)
	}

	if !pool.IsMonitoring() {
		t.Error("expected pool to be monitoring")
	}

	// Wait for a few health check cycles
	time.Sleep(100 * time.Millisecond)

	pool.StopHealthMonitoring()

	if pool.IsMonitoring() {
		t.Error("expected pool to not be monitoring after stop")
	}

	// Should have run multiple health checks
	count := atomic.LoadInt32(&checkCount)
	if count < 2 {
		t.Errorf("expected at least 2 health check calls, got %d", count)
	}

	// All workers should be healthy
	h1, _ := pool.GetWorkerHealth("worker-1")
	if h1.Status != HealthStatusHealthy {
		t.Errorf("expected worker-1 healthy, got %s", h1.Status)
	}
}

func TestWorkerPoolConcurrency(t *testing.T) {
	config := DefaultPoolConfig()
	config.MaxSize = 100
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Concurrently add 50 workers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker := &HierarchyNode{
				AgentID: "worker-" + string(rune('A'+id%26)) + string(rune('0'+id/26)),
				Role:    RoleWorker,
				Status:  "active",
			}
			if err := pool.AddWorker(worker); err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	errCount := 0
	for err := range errChan {
		t.Logf("concurrent error: %v", err)
		errCount++
	}

	if pool.Size() != 50-errCount {
		t.Errorf("expected pool size %d, got %d", 50-errCount, pool.Size())
	}
}

func TestWorkerPoolValidation(t *testing.T) {
	pool := NewWorkerPool("pool-1", "supervisor-1", DefaultPoolConfig())

	// Test nil worker
	err := pool.AddWorker(nil)
	if err == nil {
		t.Error("expected error for nil worker")
	}

	// Test empty worker ID
	err = pool.AddWorker(&HierarchyNode{AgentID: "", Role: RoleWorker})
	if err == nil {
		t.Error("expected error for empty worker ID")
	}

	// Test mark health of non-existent worker
	err = pool.MarkWorkerHealthy("non-existent")
	if err == nil {
		t.Error("expected error for non-existent worker")
	}

	err = pool.MarkWorkerUnhealthy("non-existent", "test")
	if err == nil {
		t.Error("expected error for non-existent worker")
	}
}

func TestHeartbeatHealthChecker(t *testing.T) {
	lastSeenTimes := map[string]time.Time{
		"worker-1": time.Now(),                    // Recent
		"worker-2": time.Now().Add(-5 * time.Minute), // Stale
	}

	getLastSeen := func(workerID string) (time.Time, bool) {
		t, exists := lastSeenTimes[workerID]
		return t, exists
	}

	checker := HeartbeatHealthChecker(getLastSeen, 1*time.Minute)

	ctx := context.Background()

	// Worker-1 should be healthy (recent heartbeat)
	healthy, _, err := checker(ctx, "worker-1")
	if !healthy || err != nil {
		t.Errorf("expected worker-1 to be healthy, got healthy=%v, err=%v", healthy, err)
	}

	// Worker-2 should be unhealthy (stale)
	healthy, _, err = checker(ctx, "worker-2")
	if healthy || err == nil {
		t.Errorf("expected worker-2 to be unhealthy, got healthy=%v, err=%v", healthy, err)
	}

	// Non-existent worker
	healthy, _, err = checker(ctx, "worker-3")
	if healthy || err == nil {
		t.Error("expected non-existent worker to fail health check")
	}
}

func TestWorkerPoolUpdateConfig(t *testing.T) {
	pool := NewWorkerPool("pool-1", "supervisor-1", DefaultPoolConfig())

	// Add 3 workers
	for i := 1; i <= 3; i++ {
		worker := &HierarchyNode{
			AgentID: "worker-" + string(rune('0'+i)),
			Role:    RoleWorker,
			Status:  "active",
		}
		_ = pool.AddWorker(worker)
	}

	// Update config with larger max size
	newConfig := DefaultPoolConfig()
	newConfig.MaxSize = 20
	err := pool.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	config := pool.Config()
	if config.MaxSize != 20 {
		t.Errorf("expected max size 20, got %d", config.MaxSize)
	}

	// Try to reduce below current count
	newConfig.MaxSize = 2
	err = pool.UpdateConfig(newConfig)
	if err == nil {
		t.Error("expected error when reducing max size below current count")
	}
}

func TestWorkerPoolCheckWorkerHealthNow(t *testing.T) {
	config := DefaultPoolConfig()
	config.HealthCheckTimeout = 50 * time.Millisecond
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	worker := &HierarchyNode{
		AgentID: "worker-1",
		Role:    RoleWorker,
		Status:  "active",
	}
	_ = pool.AddWorker(worker)

	// Without health checker configured
	ctx := context.Background()
	_, err := pool.CheckWorkerHealthNow(ctx, "worker-1")
	if err == nil {
		t.Error("expected error when health checker not configured")
	}

	// With health checker
	pool.SetHealthChecker(func(ctx context.Context, workerID string) (bool, int64, error) {
		return true, 10, nil
	})

	health, err := pool.CheckWorkerHealthNow(ctx, "worker-1")
	if err != nil {
		t.Fatalf("failed to check worker health: %v", err)
	}

	if health.Status != HealthStatusHealthy {
		t.Errorf("expected healthy status, got %s", health.Status)
	}

	// Check non-existent worker
	_, err = pool.CheckWorkerHealthNow(ctx, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent worker")
	}
}

func TestWorkerPoolGetAllHealth(t *testing.T) {
	config := DefaultPoolConfig()
	config.AutoRemoveUnhealthy = false
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	// Add workers
	for i := 1; i <= 3; i++ {
		worker := &HierarchyNode{
			AgentID: "worker-" + string(rune('0'+i)),
			Role:    RoleWorker,
			Status:  "active",
		}
		_ = pool.AddWorker(worker)
	}

	_ = pool.MarkWorkerHealthy("worker-1")
	_ = pool.MarkWorkerUnhealthy("worker-2", "test")

	allHealth := pool.GetAllHealth()

	if len(allHealth) != 3 {
		t.Errorf("expected 3 health records, got %d", len(allHealth))
	}

	if allHealth["worker-1"].Status != HealthStatusHealthy {
		t.Errorf("expected worker-1 healthy, got %s", allHealth["worker-1"].Status)
	}

	if allHealth["worker-2"].Status != HealthStatusUnhealthy {
		t.Errorf("expected worker-2 unhealthy, got %s", allHealth["worker-2"].Status)
	}

	if allHealth["worker-3"].Status != HealthStatusUnknown {
		t.Errorf("expected worker-3 unknown, got %s", allHealth["worker-3"].Status)
	}
}

func TestDefaultHealthChecker(t *testing.T) {
	checker := DefaultHealthChecker()

	ctx := context.Background()
	healthy, responseTime, err := checker(ctx, "any-worker")

	if !healthy {
		t.Error("expected default health checker to return healthy")
	}

	if err != nil {
		t.Errorf("expected no error from default health checker, got %v", err)
	}

	if responseTime != 1 {
		t.Errorf("expected response time 1, got %d", responseTime)
	}
}

func TestWorkerPoolSlowResponseDegrades(t *testing.T) {
	config := DefaultPoolConfig()
	config.MaxResponseTimeMs = 100 // 100ms threshold
	config.AutoRemoveUnhealthy = false
	pool := NewWorkerPool("pool-1", "supervisor-1", config)

	worker := &HierarchyNode{
		AgentID: "worker-1",
		Role:    RoleWorker,
		Status:  "active",
	}
	_ = pool.AddWorker(worker)

	// Health checker returns healthy but slow
	pool.SetHealthChecker(func(ctx context.Context, workerID string) (bool, int64, error) {
		return true, 200, nil // 200ms response time (over threshold)
	})

	ctx := context.Background()
	pool.RunHealthChecks(ctx)

	health, _ := pool.GetWorkerHealth("worker-1")
	if health.Status != HealthStatusDegraded {
		t.Errorf("expected worker to be degraded due to slow response, got %s", health.Status)
	}
}
