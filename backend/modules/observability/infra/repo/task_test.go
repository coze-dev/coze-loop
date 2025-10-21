package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	mysql "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	mysqlconv "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/convertor"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
)

type stubIDGenerator struct {
	nextID int64
	err    error
}

func (s stubIDGenerator) GenID(context.Context) (int64, error) {
	return s.nextID, s.err
}

func (s stubIDGenerator) GenMultiIDs(context.Context, int) ([]int64, error) {
	return nil, nil
}

type stubTaskDao struct {
	createTaskFunc         func(ctx context.Context, po *mysqlmodel.ObservabilityTask) (int64, error)
	updateTaskFunc         func(ctx context.Context, po *mysqlmodel.ObservabilityTask) error
	updateTaskWithOCCFunc  func(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error
	deleteTaskFunc         func(ctx context.Context, id int64, workspaceID int64, userID string) error
	getTaskFunc            func(ctx context.Context, id int64, workspaceID *int64, userID *string) (*mysqlmodel.ObservabilityTask, error)
	listTasksFunc          func(ctx context.Context, param mysql.ListTaskParam) ([]*mysqlmodel.ObservabilityTask, int64, error)
	getObjListWithTaskFunc func(ctx context.Context) ([]string, []string, []*mysqlmodel.ObservabilityTask, error)
}

func (s *stubTaskDao) CreateTask(ctx context.Context, po *mysqlmodel.ObservabilityTask) (int64, error) {
	if s.createTaskFunc != nil {
		return s.createTaskFunc(ctx, po)
	}
	return 0, nil
}

func (s *stubTaskDao) UpdateTask(ctx context.Context, po *mysqlmodel.ObservabilityTask) error {
	if s.updateTaskFunc != nil {
		return s.updateTaskFunc(ctx, po)
	}
	return nil
}

func (s *stubTaskDao) UpdateTaskWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	if s.updateTaskWithOCCFunc != nil {
		return s.updateTaskWithOCCFunc(ctx, id, workspaceID, updateMap)
	}
	return nil
}

func (s *stubTaskDao) DeleteTask(ctx context.Context, id int64, workspaceID int64, userID string) error {
	if s.deleteTaskFunc != nil {
		return s.deleteTaskFunc(ctx, id, workspaceID, userID)
	}
	return nil
}

func (s *stubTaskDao) GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*mysqlmodel.ObservabilityTask, error) {
	if s.getTaskFunc != nil {
		return s.getTaskFunc(ctx, id, workspaceID, userID)
	}
	return nil, nil
}

func (s *stubTaskDao) ListTasks(ctx context.Context, param mysql.ListTaskParam) ([]*mysqlmodel.ObservabilityTask, int64, error) {
	if s.listTasksFunc != nil {
		return s.listTasksFunc(ctx, param)
	}
	return nil, 0, nil
}

func (s *stubTaskDao) GetObjListWithTask(ctx context.Context) ([]string, []string, []*mysqlmodel.ObservabilityTask, error) {
	if s.getObjListWithTaskFunc != nil {
		return s.getObjListWithTaskFunc(ctx)
	}
	return nil, nil, nil, nil
}

type stubTaskRedisDao struct {
	getTaskFunc            func(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error)
	setTaskFunc            func(ctx context.Context, task *entity.ObservabilityTask) error
	listNonFinalTaskFunc   func(ctx context.Context, spaceID string) ([]int64, error)
	addNonFinalTaskFunc    func(ctx context.Context, spaceID string, taskID int64) error
	removeNonFinalTaskFunc func(ctx context.Context, spaceID string, taskID int64) error
}

func (s *stubTaskRedisDao) GetTask(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) {
	if s.getTaskFunc != nil {
		return s.getTaskFunc(ctx, taskID)
	}
	return nil, nil
}

func (s *stubTaskRedisDao) SetTask(ctx context.Context, task *entity.ObservabilityTask) error {
	if s.setTaskFunc != nil {
		return s.setTaskFunc(ctx, task)
	}
	return nil
}

func (s *stubTaskRedisDao) ListNonFinalTask(ctx context.Context, spaceID string) ([]int64, error) {
	if s.listNonFinalTaskFunc != nil {
		return s.listNonFinalTaskFunc(ctx, spaceID)
	}
	return nil, nil
}

func (s *stubTaskRedisDao) AddNonFinalTask(ctx context.Context, spaceID string, taskID int64) error {
	if s.addNonFinalTaskFunc != nil {
		return s.addNonFinalTaskFunc(ctx, spaceID, taskID)
	}
	return nil
}

func (s *stubTaskRedisDao) RemoveNonFinalTask(ctx context.Context, spaceID string, taskID int64) error {
	if s.removeNonFinalTaskFunc != nil {
		return s.removeNonFinalTaskFunc(ctx, spaceID, taskID)
	}
	return nil
}

func (s *stubTaskRedisDao) GetTaskCount(context.Context, int64) (int64, error) { return 0, nil }
func (s *stubTaskRedisDao) IncrTaskCount(context.Context, int64, time.Duration) (int64, error) {
	return 0, nil
}
func (s *stubTaskRedisDao) DecrTaskCount(context.Context, int64, time.Duration) (int64, error) {
	return 0, nil
}
func (s *stubTaskRedisDao) GetTaskRunCount(context.Context, int64, int64) (int64, error) {
	return 0, nil
}
func (s *stubTaskRedisDao) IncrTaskRunCount(context.Context, int64, int64, time.Duration) (int64, error) {
	return 0, nil
}
func (s *stubTaskRedisDao) DecrTaskRunCount(context.Context, int64, int64, time.Duration) (int64, error) {
	return 0, nil
}

type stubTaskRunDao struct{}

func (stubTaskRunDao) GetBackfillTaskRun(context.Context, *int64, int64) (*mysqlmodel.ObservabilityTaskRun, error) {
	return nil, nil
}
func (stubTaskRunDao) GetLatestNewDataTaskRun(context.Context, *int64, int64) (*mysqlmodel.ObservabilityTaskRun, error) {
	return nil, nil
}
func (stubTaskRunDao) CreateTaskRun(context.Context, *mysqlmodel.ObservabilityTaskRun) (int64, error) {
	return 0, nil
}
func (stubTaskRunDao) UpdateTaskRun(context.Context, *mysqlmodel.ObservabilityTaskRun) error {
	return nil
}
func (stubTaskRunDao) ListTaskRuns(context.Context, mysql.ListTaskRunParam) ([]*mysqlmodel.ObservabilityTaskRun, int64, error) {
	return nil, 0, nil
}
func (stubTaskRunDao) UpdateTaskRunWithOCC(context.Context, int64, int64, map[string]interface{}) error {
	return nil
}

type stubTaskRunRedisDao struct{}

func (stubTaskRunRedisDao) IncrTaskRunSuccessCount(context.Context, int64, int64, time.Duration) error {
	return nil
}
func (stubTaskRunRedisDao) DecrTaskRunSuccessCount(context.Context, int64, int64) error { return nil }
func (stubTaskRunRedisDao) IncrTaskRunFailCount(context.Context, int64, int64, time.Duration) error {
	return nil
}
func (stubTaskRunRedisDao) GetTaskRunSuccessCount(context.Context, int64, int64) (int64, error) {
	return 0, nil
}
func (stubTaskRunRedisDao) GetTaskRunFailCount(context.Context, int64, int64) (int64, error) {
	return 0, nil
}

func TestTaskRepoImpl_CreateTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		generator stubIDGenerator
		task      *entity.ObservabilityTask
		addErr    error
		setErr    error
		wantID    int64
		wantErr   error
		wantAdd   bool
		wantSet   bool
	}{
		{
			name:      "success",
			generator: stubIDGenerator{nextID: 101},
			task:      &entity.ObservabilityTask{ID: 101, WorkspaceID: 202},
			wantID:    101,
			wantAdd:   true,
			wantSet:   true,
		},
		{
			name:      "add non final task fail",
			generator: stubIDGenerator{nextID: 202},
			task:      &entity.ObservabilityTask{ID: 202, WorkspaceID: 303},
			addErr:    errors.New("add fail"),
			wantID:    202,
			wantErr:   errors.New("add fail"),
			wantAdd:   true,
		},
		{
			name:      "set task fail",
			generator: stubIDGenerator{nextID: 303},
			task:      &entity.ObservabilityTask{ID: 303, WorkspaceID: 404},
			setErr:    errors.New("set fail"),
			wantID:    303,
			wantErr:   errors.New("set fail"),
			wantAdd:   true,
			wantSet:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			taskDao := &stubTaskDao{
				createTaskFunc: func(ctx context.Context, po *mysqlmodel.ObservabilityTask) (int64, error) {
					assert.Equal(t, tt.generator.nextID, po.ID)
					assert.Equal(t, tt.task.WorkspaceID, po.WorkspaceID)
					return po.ID, nil
				},
			}

			redisDao := &stubTaskRedisDao{}
			redisDao.addNonFinalTaskFunc = func(ctx context.Context, spaceID string, taskID int64) error {
				assert.Equal(t, tt.task.WorkspaceID, int64FromString(spaceID))
				return tt.addErr
			}
			redisDao.setTaskFunc = func(ctx context.Context, task *entity.ObservabilityTask) error {
				assert.Equal(t, tt.task.ID, task.ID)
				return tt.setErr
			}

			repo := &TaskRepoImpl{
				TaskDao:         taskDao,
				TaskRunDao:      stubTaskRunDao{},
				TaskRedisDao:    redisDao,
				TaskRunRedisDao: stubTaskRunRedisDao{},
				idGenerator:     tt.generator,
			}

			var addCalled, setCalled bool
			originalAdd := redisDao.addNonFinalTaskFunc
			redisDao.addNonFinalTaskFunc = func(ctx context.Context, spaceID string, taskID int64) error {
				addCalled = true
				return originalAdd(ctx, spaceID, taskID)
			}
			originalSet := redisDao.setTaskFunc
			redisDao.setTaskFunc = func(ctx context.Context, task *entity.ObservabilityTask) error {
				setCalled = true
				return originalSet(ctx, task)
			}

			gotID, err := repo.CreateTask(context.Background(), tt.task)
			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantID, gotID)
			assert.Equal(t, tt.wantAdd, addCalled)
			assert.Equal(t, tt.wantSet, setCalled)
		})
	}
}

func int64FromString(s string) int64 {
	var result int64
	for i := 0; i < len(s); i++ {
		result = result*10 + int64(s[i]-'0')
	}
	return result
}

func TestTaskRepoImpl_UpdateTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		updateErr   error
		setErr      error
		expectedErr error
		expectSet   bool
	}{
		{name: "success", expectSet: true},
		{name: "update error", updateErr: errors.New("update fail"), expectedErr: errors.New("update fail")},
		{name: "set task error", setErr: errors.New("set fail"), expectedErr: errors.New("set fail"), expectSet: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			taskDao := &stubTaskDao{updateTaskFunc: func(ctx context.Context, po *mysqlmodel.ObservabilityTask) error {
				return tt.updateErr
			}}
			redisDao := &stubTaskRedisDao{setTaskFunc: func(ctx context.Context, task *entity.ObservabilityTask) error {
				return tt.setErr
			}}

			repo := &TaskRepoImpl{
				TaskDao:         taskDao,
				TaskRunDao:      stubTaskRunDao{},
				TaskRedisDao:    redisDao,
				TaskRunRedisDao: stubTaskRunRedisDao{},
			}

			var setCalled bool
			if redisDao.setTaskFunc != nil {
				original := redisDao.setTaskFunc
				redisDao.setTaskFunc = func(ctx context.Context, task *entity.ObservabilityTask) error {
					setCalled = true
					return original(ctx, task)
				}
			}

			err := repo.UpdateTask(context.Background(), &entity.ObservabilityTask{ID: 1})
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectSet, setCalled)
		})
	}
}

func TestTaskRepoImpl_DeleteTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		deleteErr    error
		removeErr    error
		expectedErr  error
		expectRemove bool
	}{
		{name: "success", expectRemove: true},
		{name: "remove error ignored", removeErr: errors.New("remove fail"), expectRemove: true},
		{name: "delete error", deleteErr: errors.New("delete fail"), expectedErr: errors.New("delete fail")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			taskDao := &stubTaskDao{deleteTaskFunc: func(ctx context.Context, id int64, workspaceID int64, userID string) error {
				return tt.deleteErr
			}}
			redisDao := &stubTaskRedisDao{removeNonFinalTaskFunc: func(ctx context.Context, spaceID string, taskID int64) error {
				return tt.removeErr
			}}

			repo := &TaskRepoImpl{
				TaskDao:         taskDao,
				TaskRunDao:      stubTaskRunDao{},
				TaskRedisDao:    redisDao,
				TaskRunRedisDao: stubTaskRunRedisDao{},
			}

			var removeCalled bool
			if redisDao.removeNonFinalTaskFunc != nil {
				original := redisDao.removeNonFinalTaskFunc
				redisDao.removeNonFinalTaskFunc = func(ctx context.Context, spaceID string, taskID int64) error {
					removeCalled = true
					return original(ctx, spaceID, taskID)
				}
			}

			err := repo.DeleteTask(context.Background(), &entity.ObservabilityTask{ID: 1, WorkspaceID: 2, CreatedBy: "user"})
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectRemove, removeCalled)
		})
	}
}

func TestTaskRepoImpl_NonFinalTaskWrappers(t *testing.T) {
	t.Parallel()

	expected := []int64{1, 2, 3}

	redisDao := &stubTaskRedisDao{}
	redisDao.listNonFinalTaskFunc = func(ctx context.Context, spaceID string) ([]int64, error) {
		assert.Equal(t, "space", spaceID)
		return expected, nil
	}
	redisDao.addNonFinalTaskFunc = func(ctx context.Context, spaceID string, taskID int64) error {
		assert.Equal(t, "space", spaceID)
		assert.Equal(t, int64(10), taskID)
		return nil
	}
	redisDao.removeNonFinalTaskFunc = func(ctx context.Context, spaceID string, taskID int64) error {
		assert.Equal(t, "space", spaceID)
		assert.Equal(t, int64(10), taskID)
		return nil
	}

	repo := &TaskRepoImpl{
		TaskDao:         &stubTaskDao{},
		TaskRunDao:      stubTaskRunDao{},
		TaskRedisDao:    redisDao,
		TaskRunRedisDao: stubTaskRunRedisDao{},
	}

	list, err := repo.ListNonFinalTask(context.Background(), "space")
	assert.NoError(t, err)
	assert.Equal(t, expected, list)

	assert.NoError(t, repo.AddNonFinalTask(context.Background(), "space", 10))
	assert.NoError(t, repo.RemoveNonFinalTask(context.Background(), "space", 10))
}

func TestTaskRepoImpl_GetTaskByRedis(t *testing.T) {
	t.Parallel()

	samplePO := &mysqlmodel.ObservabilityTask{ID: 100, WorkspaceID: 200, Name: "task"}
	convertedDO := mysqlconv.TaskPO2DO(samplePO)
	cachedTask := &entity.ObservabilityTask{ID: 1}

	tests := []struct {
		name         string
		redisFunc    func(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error)
		mysqlFunc    func(ctx context.Context, id int64, workspaceID *int64, userID *string) (*mysqlmodel.ObservabilityTask, error)
		setFunc      func(ctx context.Context, task *entity.ObservabilityTask) error
		expectErr    error
		expectMysql  bool
		expectSet    bool
		expectResult *entity.ObservabilityTask
	}{
		{
			name:         "cache hit",
			redisFunc:    func(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) { return cachedTask, nil },
			expectResult: cachedTask,
		},
		{
			name: "redis error",
			redisFunc: func(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) {
				return nil, errors.New("redis fail")
			},
			expectErr: errors.New("redis fail"),
		},
		{
			name: "mysql error",
			redisFunc: func(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) {
				return nil, nil
			},
			mysqlFunc: func(ctx context.Context, id int64, workspaceID *int64, userID *string) (*mysqlmodel.ObservabilityTask, error) {
				return nil, errors.New("mysql fail")
			},
			expectErr:   errors.New("mysql fail"),
			expectMysql: true,
		},
		{
			name:      "mysql miss",
			redisFunc: func(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) { return nil, nil },
			mysqlFunc: func(ctx context.Context, id int64, workspaceID *int64, userID *string) (*mysqlmodel.ObservabilityTask, error) {
				return nil, nil
			},
			expectMysql: true,
		},
		{
			name:      "mysql hit and cache success",
			redisFunc: func(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) { return nil, nil },
			mysqlFunc: func(ctx context.Context, id int64, workspaceID *int64, userID *string) (*mysqlmodel.ObservabilityTask, error) {
				return samplePO, nil
			},
			setFunc: func(ctx context.Context, task *entity.ObservabilityTask) error {
				assert.Equal(t, convertedDO.ID, task.ID)
				return nil
			},
			expectMysql:  true,
			expectSet:    true,
			expectResult: convertedDO,
		},
		{
			name:      "cache set fail",
			redisFunc: func(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) { return nil, nil },
			mysqlFunc: func(ctx context.Context, id int64, workspaceID *int64, userID *string) (*mysqlmodel.ObservabilityTask, error) {
				return samplePO, nil
			},
			setFunc: func(ctx context.Context, task *entity.ObservabilityTask) error {
				return errors.New("set fail")
			},
			expectErr:   errors.New("set fail"),
			expectMysql: true,
			expectSet:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			redisDao := &stubTaskRedisDao{getTaskFunc: tt.redisFunc, setTaskFunc: tt.setFunc}
			taskDao := &stubTaskDao{getTaskFunc: tt.mysqlFunc}
			repo := &TaskRepoImpl{
				TaskDao:         taskDao,
				TaskRunDao:      stubTaskRunDao{},
				TaskRedisDao:    redisDao,
				TaskRunRedisDao: stubTaskRunRedisDao{},
			}

			var mysqlCalled, setCalled bool
			if taskDao.getTaskFunc != nil {
				original := taskDao.getTaskFunc
				taskDao.getTaskFunc = func(ctx context.Context, id int64, workspaceID *int64, userID *string) (*mysqlmodel.ObservabilityTask, error) {
					mysqlCalled = true
					return original(ctx, id, workspaceID, userID)
				}
			}
			if redisDao.setTaskFunc != nil {
				original := redisDao.setTaskFunc
				redisDao.setTaskFunc = func(ctx context.Context, task *entity.ObservabilityTask) error {
					setCalled = true
					return original(ctx, task)
				}
			}

			got, err := repo.GetTaskByRedis(context.Background(), 100)
			if tt.expectErr != nil {
				assert.EqualError(t, err, tt.expectErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectMysql, mysqlCalled)
			assert.Equal(t, tt.expectSet, setCalled)
			assert.Equal(t, tt.expectResult, got)
		})
	}
}

func TestTaskRepoImpl_SetTask(t *testing.T) {
	t.Parallel()

	called := false
	redisDao := &stubTaskRedisDao{setTaskFunc: func(ctx context.Context, task *entity.ObservabilityTask) error {
		called = true
		assert.Equal(t, int64(1), task.ID)
		return nil
	}}
	repo := &TaskRepoImpl{
		TaskDao:         &stubTaskDao{},
		TaskRunDao:      stubTaskRunDao{},
		TaskRedisDao:    redisDao,
		TaskRunRedisDao: stubTaskRunRedisDao{},
	}

	assert.NoError(t, repo.SetTask(context.Background(), &entity.ObservabilityTask{ID: 1}))
	assert.True(t, called)
}
