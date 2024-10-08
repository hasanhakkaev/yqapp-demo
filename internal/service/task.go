package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/hasanhakkaev/yqapp-demo/api/tasks/v1"
	"github.com/hasanhakkaev/yqapp-demo/internal/database"
	"github.com/hasanhakkaev/yqapp-demo/internal/domain"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/time/rate"
	"sync"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	taskTypeSums   = make(map[int]uint32)
	taskTypeSumsMu sync.Mutex // Mutex to protect taskTypeSums map

	receivedTasks = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tasks_received_total",
		Help: "The total number of received tasks",
	})

	processingTasks = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tasks_processing_total",
		Help: "The total number of tasks being processed",
	})

	doneTasks = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tasks_done_total",
		Help: "The total number of tasks completed",
	})

	taskTypeCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tasks_per_type_total",
			Help: "The number of tasks per task type",
		},
		[]string{"type"},
	)

	taskValueSum = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "task_value_sum_per_type",
			Help: "The total sum of task values per task type",
		},
		[]string{"type"},
	)

	totalTasksByType = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "total_tasks_by_type",
			Help: "The total number of tasks processed by type",
		},
		[]string{"type"},
	)
)

type TaskService struct {
	v1.UnimplementedTaskServiceServer
	logger      *zap.Logger
	queries     *database.Queries
	meter       metric.Meter
	taskChannel chan *domain.Task
	taskLimiter *rate.Limiter
}

// NewTaskService initializes a new v1.TaskProducerServiceServer implementation.
func NewTaskService(logger *zap.Logger, queries *database.Queries, meter metric.Meter, taskChannel chan *domain.Task, taskLimiter *rate.Limiter) *TaskService {
	return &TaskService{
		logger:      logger,
		queries:     queries,
		meter:       meter,
		taskChannel: taskChannel,
		taskLimiter: taskLimiter,
	}
}

func (svc *TaskService) CreateTask(ctx context.Context, request *v1.CreateTaskRequest) (*v1.Task, error) {
	//taskChannel := make(chan *domain.Task, 100) // Buffered channel

	svc.logger.Log(svc.logger.Level(), "Received task creation request")

	svc.logger.Log(svc.logger.Level(), "Parsing task from API request")

	domainTask := domain.FromProtoToDomain(request.GetTask())

	domainTask.State = domain.StateRECEIVED
	domainTask.CreationTime = float64(time.Now().Unix())
	domainTask.LastUpdateTime = 0
	svc.logger.Log(svc.logger.Level(), "Filling out task information")

	taskParams := domainTask.ToTaskCreateParams()

	svc.logger.Log(svc.logger.Level(), "Persisting task in the database")

	dbTaskID, err := svc.queries.CreateTask(ctx, *taskParams)
	if err != nil {
		svc.logger.Log(svc.logger.Level(), "Failed to persist task in the database", zap.Error(err))
		return nil, status.Error(codes.Unavailable, "failed to create task")
	}

	domainTask.ID = uint32(dbTaskID)

	// After persisting task in DB
	svc.taskChannel <- domainTask

	receivedTasks.Inc()

	go svc.ConsumeTasks(svc.taskChannel, svc.taskLimiter)

	svc.logger.Log(svc.logger.Level(), "Task in the database persisted!")

	svc.logger.Log(svc.logger.Level(), "Returning created task", zap.Int("task.id", int(dbTaskID)))

	return domain.FromDomainToProto(domainTask), nil

}

// ProcessTask processes a single task, updating its state and tracking metrics.
func (svc *TaskService) ProcessTask(ctx context.Context, task *domain.Task) error {
	svc.logger.Log(svc.logger.Level(), "Handling task", zap.Int("task.id", int(task.ID)))

	// Update task state to "processing"
	_, err := svc.queries.UpdateTaskState(ctx, database.UpdateTaskStateParams{
		State:          database.StatePROCESSING,
		LastUpdateTime: float64(time.Now().Unix()),
		ID:             int32(task.ID),
	})
	if err != nil {
		svc.logger.Error("Failed to update task to processing", zap.Error(err))
		return status.Error(codes.Internal, "Failed to update task to processing")
	}

	// Increment processing tasks metric
	processingTasks.Inc()

	// Simulate processing by sleeping for task's value in milliseconds
	time.Sleep(time.Duration(task.Value) * time.Millisecond)

	if errors.Is(ctx.Err(), context.Canceled) {
		svc.logger.Log(svc.logger.Level(), "Request is canceled")
		return status.Error(codes.Canceled, "Request is canceled")
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		svc.logger.Log(svc.logger.Level(), "Request deadline exceeded")
		return status.Error(codes.DeadlineExceeded, "Request deadline exceeded")
	}

	// Update task state to "done"
	_, err = svc.queries.UpdateTaskState(ctx, database.UpdateTaskStateParams{
		State:          database.StateDONE,
		LastUpdateTime: float64(time.Now().Unix()),
		ID:             int32(task.ID),
	})
	if err != nil {
		svc.logger.Error("Failed to update task to done", zap.Error(err))
		return status.Error(codes.Internal, "Failed to update task to done")
	}

	// Update metrics
	receivedTasks.Dec()
	processingTasks.Dec()

	doneTasks.Inc()

	taskTypeLabel := fmt.Sprintf("%d", task.Type)
	totalTasksByType.WithLabelValues(taskTypeLabel).Inc()

	taskTypeCount.WithLabelValues(fmt.Sprintf("%d", task.Type)).Inc()
	taskValueSum.WithLabelValues(fmt.Sprintf("%d", task.Type)).Add(float64(task.Value))

	// Use mutex to protect access to taskTypeSums
	taskTypeSumsMu.Lock()
	taskTypeSums[int(task.Type)] += task.Value
	taskTypeSumsMu.Unlock()

	svc.logger.Log(svc.logger.Level(), "Task processed", zap.Int("id", int(task.ID)),
		zap.Int("type", int(task.Type)), zap.Int("value", int(task.Value)))

	svc.logger.Log(svc.logger.Level(), "Task's content: ", zap.Any("task", task))

	svc.logger.Log(svc.logger.Level(), "Sum of task values of type : ", zap.Int("type:", int(task.Type)), zap.Int("Sum", int(taskTypeSums[int(task.Type)])))

	return nil
}

// ConsumeTasks handles incoming tasks with a rate limiter.
func (svc *TaskService) ConsumeTasks(taskChannel <-chan *domain.Task, limiter *rate.Limiter) {
	for task := range taskChannel {
		// Apply rate limiting
		err := limiter.Wait(context.Background())
		if err != nil {
			svc.logger.Fatal("Rate limiter error", zap.Error(err))
		}

		// Process each task
		err = svc.ProcessTask(context.Background(), task)
		if err != nil {
			return
		}
	}
}
