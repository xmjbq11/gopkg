package bstd

import (
	"context"
	"sync"
)

// Job func() error
type Job func() error

// ContextJob func(context.Context) error
type ContextJob func(context.Context) error

// ErrorHandler func(error)
type ErrorHandler func(error)

// DisposeFn 是Job的alias，某个方法返回DisposeFn表示需要调用者
// 负责调用来回收响应的资源
type DisposeFn = Job

// IgnoreError 忽略错误ErrorHandler
func IgnoreError(_ error) {}

// Noop 空Job
func Noop() error {
	return nil
}

// GoChan 在goroutine中执行job，并返回异步错误
// 返回channel至多包含一个错误，不会被关闭
func GoChan(job Job) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- job()
	}()

	return ch
}

// GoWG 在goroutine中执行job，并对应调整wg，错误由传入handler处理
func GoWG(job Job, catch ErrorHandler, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		catch(job())
	}()
}

// RunWithContext 阻塞至job执行完毕或context退出。
// **注意当context退出，没有机会回收执行job的goroutine，有泄露风险**
func RunWithContext(ctx context.Context, job Job) error {
	ch := GoChan(job)
	select {
	case err := <-ch:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// RunAfterContextDone (在独立goroutine中) 等待context结束后，执行job，错误由catch处理
func RunAfterContextDone(ctx context.Context, job Job, catch ErrorHandler) {
	go func() {
		<-ctx.Done()
		catch(job())
	}()
}

// RunAfterContextDoneWG (在独立goroutine中) 等待context结束后，执行job，错误由catch处理，阻塞wg
func RunAfterContextDoneWG(ctx context.Context, job Job, catch ErrorHandler, wg *sync.WaitGroup) {
	GoWG(func() error {
		<-ctx.Done()
		return job()
	}, catch, wg)
}

// WrapJobContext 将job封装为可感知context。
// **注意当context退出，没有机会回收执行job的goroutine，有泄露风险**
func WrapJobContext(job Job) ContextJob {
	return func(ctx context.Context) error {
		ch := GoChan(job)
		select {
		case err := <-ch:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	}
}

// WithContext 固定contextJob执行时的context
func WithContext(ctx context.Context, cj ContextJob) Job {
	return func() error {
		return cj(ctx)
	}
}

// WithErrorHandler 将job和errorHandler组合成一个裸func()
func WithErrorHandler(job Job, handler ErrorHandler) func() {
	return func() {
		handler(job())
	}
}

// CJobList 维护一组contextJob
type CJobList struct {
	list []ContextJob
	lock sync.RWMutex
}

func NewCJobList(jobs ...ContextJob) *CJobList {
	return &CJobList{
		list: jobs,
	}
}

// RunOneByOne 在本goroutine依次执行，遇到error直接返回
func (l *CJobList) RunOneByOne(ctx context.Context) error {
	l.lock.RLock()
	defer l.lock.RUnlock()

	for _, job := range l.list {
		err := job(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunOneByOne 在本goroutine依次执行，遇到error执行catcher后继续
func (l *CJobList) RunOneByOneCatch(ctx context.Context, catch ErrorHandler) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	for _, job := range l.list {
		catch(job(ctx))
	}
}

// RunOneByOneReset 在本goroutine依次执行并清空，遇到error直接返回
func (l *CJobList) RunOneByOneReset(ctx context.Context) error {
	l.lock.Lock()
	defer l.lock.Unlock()

	for _, job := range l.list {
		err := job(ctx)
		if err != nil {
			return err
		}
	}
	l.list = nil

	return nil
}

// RunOneByOneCatchReset 在本goroutine依次执行并清空，遇到error执行catcher后继续
func (l *CJobList) RunOneByOneCatchReset(ctx context.Context, catch ErrorHandler) {
	l.lock.Lock()
	defer l.lock.Unlock()

	for _, job := range l.list {
		catch(job(ctx))
	}
	l.list = nil
}

// RunAll 并发执行全部任务，由catcher处理错误
func (l *CJobList) RunAll(ctx context.Context, catch ErrorHandler) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	wg := new(sync.WaitGroup)
	for _, job := range l.list {
		GoWG(WithContext(ctx, job), catch, wg)
	}
	wg.Wait()
}

// Append 追加任务
func (l *CJobList) Append(cj ContextJob) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if cj == nil {
		panic("nil contextJob")
	}
	l.list = append(l.list, cj)
}

// Reset 清除所有任务
func (l *CJobList) Reset() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.list = nil
}
