package tasker // import "github.com/webdeskltd/tasker"

//import "github.com/webdeskltd/log"
//import "github.com/webdeskltd/debug"
import (
	"fmt"
)

// Do Реализация воркера, горутина
func (w *worker) Do() {
	var t *task
	var r *result
	var done bool

	defer func() { w.Done <- true }()
	for {
		if done && len(w.Parent.ChanIn) == 0 {
			break
		}
		select {
		case <-w.Shutdown:
			done = true
		case t = <-w.Parent.ChanIn:
			r = &result{Task: t}
			if w.Parent.WorkerFn != nil {
				if r.Error = w.Run(w.Parent.WorkerFn, t); r.Error != nil {
					t.CountError++
				}
			}
			w.Parent.ChanOut <- r
		}
	}
}

// Run Безопасный запуск внешнего воркера
func (w *worker) Run(f WorkerFunc, t *task) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("Recovery panic call external worker: %v", e)
			return
		}
	}()
	err = f(t.Body)
	return
}
