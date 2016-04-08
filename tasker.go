package tasker // import "github.com/webdeskltd/tasker"

//import "github.com/webdeskltd/debug"
//import "github.com/webdeskltd/log"
import (
	"container/list"
	"fmt"
	"runtime"
)

// NewTasker Function create new tasker implementation
func NewTasker() Tasker {
	var tsk = new(implementation)

	// Default number of concurent task
	tsk.Concurrent(runtime.NumCPU())

	// Initialization task list
	tsk.Tasks = list.New()

	// Входящие задачи
	tsk.ChanIn = make(chan *task, 1)

	// Выполненные задачи
	tsk.ChanOut = make(chan *result, 1000)

	// Interrupt
	tsk.ChanInterrupt = make(chan interface{}, 1)

	return tsk
}

// Concurrent Number of concurent task
// Устанавливать и менять значения можно пока только на не запущенном tasker
func (tsk *implementation) Concurrent(n int) Tasker {
	tsk.Lock()
	defer tsk.Unlock()
	if !tsk.isWork {
		tsk.ConcurrentProcesses = n
	}
	return tsk
}

// Bootstrap Установка функции которая будет запущена до начала выполнения задач
func (tsk *implementation) Bootstrap(fn BootstrapFunc) Tasker {
	tsk.Lock()
	defer tsk.Unlock()
	tsk.BootstrapFn = fn
	return tsk
}

// Worker Установка функции обрабатывающей задачи
func (tsk *implementation) Worker(fn WorkerFunc) Tasker {
	tsk.Lock()
	defer tsk.Unlock()
	tsk.WorkerFn = fn
	return tsk
}

// RetryIfError Повторить запуск задачи если Worker вернул ошибку, но не более N раз. По умолчанию не повторять
func (tsk *implementation) RetryIfError(n int) Tasker {
	tsk.Lock()
	defer tsk.Unlock()
	tsk.RetryCount = n
	return tsk
}

// AddTasks Добавление среза объектов задач в очередь выполнения
func (tsk *implementation) AddTasks(tasks []interface{}) (err error) {
	for n := range tasks {
		if err = tsk.AddTask(tasks[n]); err != nil {
			return
		}
	}
	return
}

// AddTask Добавление одного объектов задач в очередь выполнения
func (tsk *implementation) AddTask(t interface{}) (err error) {
	tsk.Lock()
	defer tsk.Unlock()
	if t == nil {
		err = fmt.Errorf("Error, task is nil")
		return
	}
	tsk.Tasks.PushBack(&task{Body: t})
	return
}

// Clean Очистка всех задач в очереди
func (tsk *implementation) Clean() Tasker {
	tsk.Lock()
	defer tsk.Unlock()
	tsk.Tasks.Init()
	return tsk
}

// GetTasksNumber Возвращает количество не завершенных задач (ожидающих выполнения или еще выполняющихся)
func (tsk *implementation) GetTasksNumber() int { return tsk.Tasks.Len() }

// Error Крайняя ошибка
func (tsk *implementation) Error() error {
	tsk.Lock()
	defer tsk.Unlock()
	return tsk.Err
}

// Wait Ожидание окончания выполнения всех задач
// Функция блокируется до окончания выполнени всех задач
func (tsk *implementation) Wait() Tasker {
	tsk.WorkerWG.Wait()
	return tsk
}

// Interrupt Прерывания выполнения задач. Новые задачи перестают запускаться на выполнение, уже запущенные задачи будут выполнены
func (tsk *implementation) Interrupt() Tasker {
	tsk.Lock()
	defer tsk.Unlock()
	if len(tsk.ChanInterrupt) == 0 {
		tsk.ChanInterrupt <- true
	}
	return tsk
}
