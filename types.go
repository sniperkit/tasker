package tasker // import "github.com/webdeskltd/tasker"

import (
	"container/list"
	"sync"
)

// Tasker is an interface
type Tasker interface {
	AddTasks(tasks []interface{}) error // Добавление среза объектов задач в очередь выполнения
	AddTask(task interface{}) error     // Добавление одного объектов задач в очередь выполнения
	Bootstrap(BootstrapFunc) Tasker     // Установка функции которая будет запущена до начала выполнения задач
	Concurrent(int) Tasker              // Concurrent Number of concurent task
	Clean() Tasker                      // Очистка всех задач в очереди за исключением выполняющихся в текущее время
	Error() error                       // Последняя возникшая ошибка
	GetTasksNumber() int                // Возвращает количество не завершенных задач (ожидающих выполнения или еще выполняющихся)
	Interrupt() Tasker                  // Прерывания выполнения задач. Новые задачи перестают запускаться на выполнение, уже запущенные задачи будут выполнены
	IsWork() bool                       // =true - tasker выполняет задачи, =false - tasker закончил выполнение всех задач, все goroutines навершены
	Run() Tasker                        // Запуск выполнения задач без ожидания, функция возвращает выполнение после запуска контроллера задач в отдельном процессе
	RetryIfError(int) Tasker            // Повторить запуск задачи если Worker вернул ошибку, но не более N раз. По умолчанию не повторять
	Worker(WorkerFunc) Tasker           // Установка функции обрабатывающей задачи
	Wait() Tasker                       // Ожидание окончания выполнения всех задач, функция блокируется до окончания выполнени всех задач
}

// implementation is an tasker implementation
type implementation struct {
	ConcurrentProcesses int              // Максимальное количество одновременно выполняющихся задач
	Err                 error            // Последняя ошибка
	BootstrapFn         BootstrapFunc    // Функция предпусковой обработки данных для задач
	WorkerFn            WorkerFunc       // Функция обрабатывающая задачу
	Tasks               *list.List       // Список задач/данных ожидающих выполнения/обработки
	isWork              bool             // =true - tasker запущен и работает, =false - tasker остановлен
	ChanIn              chan *task       // Канал задач для воркера
	ChanOut             chan *result     // Выполненные задачи
	ChanInterrupt       chan interface{} // Прерывание выполнения задач
	WorkerPool          []worker         // Запущенные работники
	WorkerWG            sync.WaitGroup   // Лок ожидания завершения работников
	RetryCount          int              // Количество повторов запуска задачи в случае ошибки. По умолчанию 0 - не перезапускать

	sync.Mutex // Безопасненько всё делаем
}

type worker struct {
	ID       int              // Номер работника
	Shutdown chan interface{} // Сигнал завершения горутины
	Done     chan interface{} // Сигнал горутина завершена
	Parent   *implementation  // Родительский объект
}

// Структура объекта задачи
type task struct {
	Body       interface{} // Переданный извне объект задачи
	InWork     bool        // =true - задача находится в работе, =false - задача находится в очереди ожидания
	Prelude    bool        // =true - задача была обработана BootstrapFunc
	CountError int         // Количество попыток выполнить задачу завершившихся ошибкой

	sync.Mutex // Безопасненько всё делаем
}

// Структура объекта результата задачи
type result struct {
	Task  *task // Задача
	Error error // Ошибка возвращённая функцией выполнявшей задачу
}

// BootstrapFunc Тип функции которая будет запущена до начала выполнения задач
type BootstrapFunc func([]interface{}) error

// WorkerFunc Тип функции выполняющей задачу
type WorkerFunc func(interface{}) error
