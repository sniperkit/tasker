// +build none

package main

//import "gopkg.in/webnice/debug.v1"
//import "gopkg.in/webnice/log.v2"
import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"time"

	"github.com/webdeskltd/tasker"
)

// Task The structure of the object with the data for the task
type Task struct {
	Sleep          time.Duration
	TextOnColplete string
}

var doneTask int

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	Main()

	// Test automatic run destructor (finalizer)
	runtime.Gosched()
	time.Sleep(time.Second / 2)
}

// generateTasksData Create a test data slice for tasks
func generateTasksData(consignment string) (t []interface{}) {
	for i := 0; i < 25; i++ {
		t = append(t, &Task{
			TextOnColplete: fmt.Sprintf("task %d[%s]", i, consignment),
			Sleep:          time.Microsecond,
		})
	}
	return
}

// Main test
func Main() {
	var tasks tasker.Tasker
	var err error

	// Создание очереди задач
	tasks = tasker.NewTasker().
		// Указание максимального количества паралельно выполняемых задач
		Concurrent(15).

		// Функция которая будет запущена до начала выполнения задач (tasker.BootstrapFunc)
		Bootstrap(beforeBegin).

		// Функция обработки задач не возвращающих результат (tasker.WorkerFunc)
		Worker(worker).

		// Десять попыток выполнить задачу в случае если worker вернёт ошибку
		RetryIfError(100)

	// Проверка как работает IsWork()
	go func() {
		for {
			if tasks.IsWork() == true {
				log.Printf(".... Работает")
			} else {
				log.Printf(".... Не работает")
			}
			time.Sleep(time.Second / 4)
		}
	}()

	// Добавление данных для задач
	var datas = generateTasksData("part 1")
	log.Printf("Партия задач 1: %d\n", len(datas))
	if err = tasks.AddTasks(datas); err != nil {
		log.Fatalf("Error add tasks: %s", err.Error())
	}

	log.Printf("В очереди задач: %d\n", tasks.GetTasksNumber())

	// Запуск задач на выполнение и сразу считывание последней ошибки
	if err = tasks.Run().Error(); err != nil {
		log.Fatalf("Error tun tasker: %s", err.Error())
	}

	time.Sleep(time.Second / 2)
	tasks.Interrupt().Wait()
	log.Printf(" - Прервались, в очереди задач: %d\n", tasks.GetTasksNumber())
	log.Printf(" - 3 секунды и дальше...")
	time.Sleep(time.Second * 3)
	tasks.Run()

	// Добавляем еще задач
	datas = generateTasksData("part 2")
	log.Printf("Партия задач 2: %d\n", len(datas))
	if err = tasks.AddTasks(datas); err != nil {
		log.Printf("Error add tasks: %s", err.Error())
	}

	log.Printf("В очереди задач: %d\n", tasks.GetTasksNumber())
	tasks.Wait()

	log.Printf("Выполнено задач: %d", doneTask)
	log.Printf("Не выполнено задач: %d\n", tasks.GetTasksNumber())
	if tasks.GetTasksNumber() > 0 {
		tasks.Run().Wait()
		log.Printf("Выполнено задач: %d", doneTask)
	}

}

// beforeBegin Функции которая будет запущена до начала выполнения задач
func beforeBegin(in []interface{}) (err error) {
	for i := range in {
		var task = in[i].(*Task)

		task.Sleep = time.Millisecond * time.Duration(rand.Intn(1000))
		task.TextOnColplete = task.TextOnColplete + `[beforeBegin ok]`
	}
	return
}

// worker Функция выполняющая задачу
func worker(in interface{}) (err error) {
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

	time.Sleep(task.Sleep)
	log.Printf("+ %s done\n", task.TextOnColplete)
	doneTask++

	return
}
