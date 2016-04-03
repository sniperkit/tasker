package tasker // import "github.com/webdeskltd/tasker"

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// Task The structure of the object with the data for the task
type Task struct {
	Sleep          time.Duration
	TextOnColplete string
}

type WorkNotWork struct {
	Work    int64
	NotWork int64
}

var doneTask int
var TestWorkNotWork WorkNotWork

// generateTasksData Create a test data slice for tasks
func generateTasksData(consignment string) (t []interface{}) {
	for i := 0; i < 100; i++ {
		t = append(t, &Task{
			TextOnColplete: fmt.Sprintf("task %d[%s]", i, consignment),
			Sleep:          time.Microsecond,
		})
	}
	return
}

func TestFoolProtection(t *testing.T) {
	var tasks Tasker
	var err error

	// Создание очереди задач
	tasks = NewTasker().
		// Указание максимального количества паралельно выполняемых задач
		Concurrent(0).

		// Функция которая будет запущена до начала выполнения задач (tasker.BootstrapFunc)
		Bootstrap(func(in []interface{}) (err error) {
			panic("Hi jack!")
		}).

		// Десять попыток выполнить задачу в случае если worker вернёт ошибку
		RetryIfError(100)

	// Test fool protection
	if err = tasks.AddTasks([]interface{}{nil}); err == nil {
		t.Fatalf("Error add nil task")
		return
	}

	// Concurrent = 0
	err = tasks.Run().Error()
	if strings.Index(fmt.Sprintf("%v", err), "An invalid value in parallel running processes") != 0 {
		t.Fatalf("Error check 'An invalid value in parallel running processes'")
	}
	tasks.Concurrent(15)

	// Worker function
	err = tasks.Run().Error()
	if strings.Index(fmt.Sprintf("%v", err), "Not specified Worker function") != 0 {
		t.Fatalf("Error check 'Not specified Worker function'")
	}
	tasks.Worker(func(in interface{}) error { return nil })

	var datas = generateTasksData("part 1")
	if err = tasks.AddTasks(datas); err != nil {
		t.Fatalf("Error add tasks: %s", err.Error())
		return
	}

	// BootstrapFunc
	err = tasks.Run().Error()
	if strings.Index(fmt.Sprintf("%v", err), "Recovery panic call external BootstrapFunc") != 0 {
		t.Fatalf("Error check 'Recovery panic call external BootstrapFunc'")
	}
	tasks.Bootstrap(nil)

	a := tasks.GetTasksNumber()
	b := tasks.Clean().GetTasksNumber()
	if a != 100 || b != 0 {
		t.Fatalf("Error in task management")
	}

	if err = tasks.Run().Error(); err != nil {
		t.Fatalf("Unknown error: %v", err)
	}

	if err = tasks.Wait().Error(); err != nil {
		t.Fatalf("Unknown error: %v", err)
	}
}

// TestTasker Main test
func TestTasker(t *testing.T) {
	var tasks Tasker
	var err error

	// Создание очереди задач
	tasks = NewTasker().
		// Указание максимального количества паралельно выполняемых задач
		Concurrent(70).

		// Функция которая будет запущена до начала выполнения задач (tasker.BootstrapFunc)
		Bootstrap(fnTestMainBeforeBegin).

		// Функция обработки задач не возвращающих результат (tasker.WorkerFunc)
		Worker(fnTestMainWorker).

		// Попытки выполнить задачу в случае если worker вернёт ошибку
		RetryIfError(1000)

	// Проверка как работает IsWork()
	go func() {
		for {
			if tasks.IsWork() == true {
				TestWorkNotWork.Work++
			} else {
				TestWorkNotWork.NotWork++
			}
			time.Sleep(time.Second / 4)
			if TestWorkNotWork.Work > 5 && TestWorkNotWork.NotWork > 5 {
				break
			}
		}
	}()

	// Добавление данных для задач
	var datas = generateTasksData("part 1")
	t.Logf("Партия задач 1: %d\n", len(datas))
	if err = tasks.AddTasks(datas); err != nil {
		t.Fatalf("Error add tasks: %s", err.Error())
		return
	}

	t.Logf("В очереди задач: %d\n", tasks.GetTasksNumber())

	// Запуск задач на выполнение и сразу считывание последней ошибки
	if err = tasks.Run().Error(); err != nil {
		t.Fatalf("Error tun tasker: %s", err.Error())
		return
	}

	// Проверка already running
	if err = tasks.Run().Error(); err == nil {
		t.Fatal("Error check 'Tasker already running'")
		return
	}

	time.Sleep(time.Second / 4)
	tasks.Interrupt().Wait()
	t.Logf(" - Прервались, в очереди задач: %d\n", tasks.GetTasksNumber())
	t.Logf(" - секунда и дальше...")
	time.Sleep(time.Second)

	tasks.Run()

	// Добавляем еще задач
	datas = generateTasksData("part 2")
	t.Logf("Партия задач 2: %d\n", len(datas))
	_ = tasks.AddTasks(datas)
	time.Sleep(time.Second * 3)

	t.Logf("В очереди задач: %d\n", tasks.GetTasksNumber())
	tasks.Wait()

	t.Logf("Выполнено задач: %d", doneTask)
	t.Logf("Не выполнено задач из за прерывания и добавления: %d\n", tasks.GetTasksNumber())
	if tasks.GetTasksNumber() > 0 {
		tasks.Run().Wait()
		t.Logf("Выполнено задач: %d", doneTask)
	}

	// Количество поставленных и выполненных задач не совпадёт если исчерпаны попытки выполнения!
	if doneTask+tasks.GetTasksNumber() != 200 {
		t.Fatalf("Test failed, loss tasks: done=%d, undone=%d", doneTask, tasks.GetTasksNumber())
	}

}

// fnTestMainBeforeBegin Функции которая будет запущена до начала выполнения задач
func fnTestMainBeforeBegin(in []interface{}) (err error) {
	for i := range in {
		var task = in[i].(*Task)

		task.Sleep = time.Millisecond * time.Duration(rand.Intn(1000))
		task.TextOnColplete = task.TextOnColplete + `[beforeBegin ok]`
	}
	return
}

// fnTestMainWorker Функция выполняющая задачу
func fnTestMainWorker(in interface{}) (err error) {
	var task = in.(*Task)

	if rand.Intn(100) < 15 {
		task.TextOnColplete += " panic()"
		panic("hi jack!")
	}

	if rand.Intn(100) > 40 {
		err = fmt.Errorf("Test error")
	}

	if err != nil {
		task.TextOnColplete += " err()"
		return
	}

	// time.Sleep(task.Sleep)
	doneTask++

	return
}
