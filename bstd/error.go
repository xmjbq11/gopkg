package bstd

import (
	"errors"
	"fmt"
	"log"
)

// MustNoError 如果err非nil，log后退出
func MustNoError(err error) {
	if err != nil {
		log.Fatalf("%+v\n", err)
	}
}

// ErrorHandlerToWChan 返回一个写chan，进入的error对象会被对应error handler处理
// 调用者负责关闭chan
func ErrorHandlerToWChan(hdl ErrorHandler) chan<- error {
	ch := make(chan error)
	go func() {
		for err := range ch {
			if err != nil {
				hdl(err)
			}
		}
	}()

	return ch
}

// EatError 如果任意ignoredError满足errors.Is的探测，则返回nil，否则返回原本的error
func EatError(err error, ignoredErrors ...error) error {
	if err == nil {
		return nil
	}
	for _, ignoredError := range ignoredErrors {
		if errors.Is(err, ignoredError) {
			return nil
		}
	}

	return err
}

type stdError string

func (s stdError) Error() string {
	return string(s)
}

// Errors 收集多个可能发生的error整体处理
type Errors []error

// Error 实现error接口，注意即使长度为0同样满足error接口
func (ep *Errors) Error() string {
	return fmt.Sprintf("<%d errors>", ep.Len())
}

// Details 实现emperror.dev/errors details接口
func (ep *Errors) Details() []interface{} {
	details := AnyMap{}
	for i, err := range *ep {
		details.Set(fmt.Sprintf("err#%d", i), err)
	}
	return details
}

// Collect 收集err错误
func (ep *Errors) Collect(err error) {
	if err != nil {
		*ep = append(*ep, err)
	}
}

// Len 返回长度
func (ep *Errors) Len() int {
	if ep == nil {
		return 0
	}
	return len(*ep)
}

// Err 如果有内容，返回自身，否则nil
func (ep *Errors) Err() error {
	if ep.Len() == 0 {
		return nil
	}

	return ep
}
