package tasker // import "github.com/webdeskltd/tasker"

//import "github.com/webdeskltd/log"
//import "github.com/webdeskltd/debug"
import (
	"container/list"
	"fmt"
	"sync"
)

// Run Запуск выполнения задач без ожидания
// Функция возвращает выполнение после запуска контроллера задач
func (tsk *implementation) Run() Tasker {
	tsk.Lock()
	defer tsk.Unlock()

	var i int

	tsk.Err = tsk.CanRun()
	if tsk.Err != nil {
		return tsk
	}

	// Предварительная обработка всех задач функцией BootstrapFunc
	// Если при первом запуске будет ошибка то ничего не стартует
	// В случае ошибки при запуске в ходе работы, будет прерывание
	if tsk.PreludeTasks(); tsk.Err != nil {
		return tsk
	}

	tsk.InWork = true

	// Создание работников
	for i = 0; i < tsk.ConcurrentProcesses; i++ {
		tsk.WorkerPool = append(tsk.WorkerPool, worker{
			ID:       i,
			Parent:   tsk,
			Shutdown: make(chan interface{}, 1),
			Done:     make(chan interface{}, 1),
		})
	}

	// Запуск всех работников
	for i = range tsk.WorkerPool {
		tsk.WorkerWG.Add(1)
		go func(wg *sync.WaitGroup, w *worker) {
			defer wg.Done()
			w.Do()
		}(&tsk.WorkerWG, &tsk.WorkerPool[i])
	}

	// Запуск менеджера
	tsk.WorkerWG.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		defer func() { tsk.InWork = false }()
		tsk.Manager()
		// Отправка всем сигнала завершения
		for i := range tsk.WorkerPool {
			tsk.WorkerPool[i].Shutdown <- true
		}
		// Ждём от всех ответ о завершении
		for i := range tsk.WorkerPool {
			<-tsk.WorkerPool[i].Done
		}
	}(&tsk.WorkerWG)

	return tsk
}

// CanRun Проверка возможности запуска таскера
func (tsk *implementation) CanRun() (err error) {
	// Количество паралельных процессов долно быть больше 0
	if tsk.ConcurrentProcesses <= 0 {
		err = fmt.Errorf("An invalid value in parallel running processes: %d", tsk.ConcurrentProcesses)
		return
	}

	// Таскер не должен быть уже запущенным
	if tsk.InWork {
		err = fmt.Errorf("Tasker already running")
		return
	}

	// Функция выполнения задач не должна быть пустой
	if tsk.WorkerFn == nil {
		err = fmt.Errorf("Not specified Worker function")
		return
	}

	return
}

// Manager Процесс поставки данных работникам, получения и обработки результатов
// Manager завершается когда кончились задачи, за собой гасит всех работников
func (tsk *implementation) Manager() {
	var err error
	var r *result
	var interrupt bool

	tsk.Lock()
	defer tsk.Unlock()

	for {
		// Предварительная обработка всех задач функцией BootstrapFunc
		if tsk.PreludeTasks(); tsk.Err != nil {
			interrupt = true
		}

		// Задачи кончились и в исходящем канале пусто, можно выходить (gocyclo > 10) fuck!
		if tsk.CanExit(interrupt, err) {
			break
		}

		select {
		case <-tsk.ChanInterrupt:
			interrupt = true
		case r = <-tsk.ChanOut:
			tsk.TaskResult(r)
		default:
			// Отправка одной задачи в канал, если он заполнен меньше чем на количество паралельных процессов
			if len(tsk.ChanIn) < tsk.ConcurrentProcesses {
				err = tsk.PushNextTask()
			}
		}
	}
}

// CanExit Определяем можно ли выйти
func (tsk *implementation) CanExit(interrupt bool, err error) (ret bool) {
	if len(tsk.ChanIn) == 0 && len(tsk.ChanOut) == 0 && tsk.Tasks.Len() == 0 && err != nil || interrupt {
		ret = true
	}
	return
}

// TaskResult Обработка результата
func (tsk *implementation) TaskResult(r *result) {
	var elm *list.Element
	var item *task

	// Поиск
	for elm = tsk.Tasks.Front(); elm != nil; elm = elm.Next() {
		if elm.Value.(*task) != r.Task {
			continue
		}
		if r.Error != nil && tsk.RetryCount > r.Task.CountError {
			item = elm.Value.(*task)
			item.Lock()
			item.InWork = false
			item.Unlock()
			continue
		}
		tsk.Tasks.Remove(elm)
	}
}

// PreludeTasks Выполнение над задачами функции BootstrapFunc, если такая установлена
func (tsk *implementation) PreludeTasks() {
	var elm *list.Element
	var item *task
	var items []*task
	var i int

	for elm = tsk.Tasks.Front(); elm != nil; elm = elm.Next() {
		if elm.Value.(*task).Prelude == false {
			item = elm.Value.(*task)
			item.Lock()
			item.InWork = true
			item.Prelude = true
			item.Unlock()
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return
	}
	tsk.Err = tsk.SafeCallBootstrapFunc(items)
	for i = range items {
		items[i].Lock()
		items[i].InWork = false
		items[i].Prelude = true
		items[i].Unlock()
	}
}

// SafeCallBootstrapFunc Безопасный запуск внешней функции
func (tsk *implementation) SafeCallBootstrapFunc(items []*task) (err error) {
	var data []interface{}
	var i int

	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("Recovery panic call external BootstrapFunc: %v", e)
			return
		}
	}()

	for i = range items {
		data = append(data, items[i].Body)
	}
	if tsk.BootstrapFn != nil {
		err = tsk.BootstrapFn(data)
	}
	return
}

// PushNextTask Получение первой свободной задачи, если результат nil, задач больше нет
func (tsk *implementation) PushNextTask() (err error) {
	var elm *list.Element
	var item *task

	for elm = tsk.Tasks.Front(); elm != nil; elm = elm.Next() {
		if !elm.Value.(*task).InWork && elm.Value.(*task).Prelude {
			item = elm.Value.(*task)
			item.Lock()
			item.InWork = true
			item.Unlock()
			break
		}
	}
	if item == nil {
		err = fmt.Errorf("No new task")
		return
	}
	tsk.ChanIn <- item

	return
}

// IsWork Текущее состояние выполнения задач
// =true - tasker выполняет задачи, =false - tasker закончил выполнение всех задач, все goroutines навершены
func (tsk *implementation) IsWork() bool {
	tsk.Lock()
	defer tsk.Unlock()
	return tsk.InWork
}
