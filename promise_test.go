package promise

import (
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPromise(t *testing.T) {
	t.Run("Calls Callback Once", func(t *testing.T) {
		var i atomic.Int32

		New(func(resolve func(any), reject func(error)) {
			i.Add(1)
		})

		assert.Eventually(t, func() bool {
			return i.Load() > 0
		}, 50*time.Millisecond, time.Millisecond)

		assert.Never(t, func() bool {
			return i.Load() > 1
		}, 50*time.Millisecond, time.Millisecond)
	})

	t.Run("Resolve", func(t *testing.T) {
		p := New(func(resolve func(int), reject func(error)) {
			resolve(1)
		})

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 1, p)
			assertNoErr(t, p)
		}
	})

	t.Run("Reject", func(t *testing.T) {
		err := errors.New("mock")
		p := New(func(resolve func(any), reject func(error)) {
			reject(err)
		})

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertErr(t, err, p)
			assertNoVal(t, p)
		}
	})

	t.Run("Reject Without Error", func(t *testing.T) {
		p := New(func(resolve func(any), reject func(error)) {
			reject(nil)
		})

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertErr(t, ErrRejected, p)
			assertNoVal(t, p)
		}
	})

	t.Run("Callback Panic", func(t *testing.T) {
		tests := []struct {
			name     string
			panicVal any
			wantErr  error
		}{
			{name: "Error", panicVal: errors.New("mock"), wantErr: fmt.Errorf("panic: %w", errors.New("mock"))},
			{name: "String", panicVal: "panicking", wantErr: fmt.Errorf("panic: panicking")},
			{name: "Stringer", panicVal: stringerImpl{"panicking"}, wantErr: fmt.Errorf("panic: panicking")},
			{name: "Arbitrary", panicVal: 100, wantErr: fmt.Errorf("panic: 100")},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var p *Promise[any]

				if !assert.NotPanics(t, func() {
					p = New(func(resolve func(any), reject func(error)) {
						panic(tt.panicVal)
					})
				}) {
					return
				}

				if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
					assertErr(t, tt.wantErr, p)
					assertNoVal(t, p)
				}
			})
		}
	})

	t.Run("Resolves Once", func(t *testing.T) {
		p := New(func(resolve func(int), reject func(error)) {
			resolve(1)
			resolve(2)
		})

		if !assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			return
		}

		if !assertVal(t, 1, p) {
			return
		}

		assert.Never(t, func() bool {
			return !valEqual(1, p)
		}, 50*time.Millisecond, time.Millisecond)
	})

	t.Run("Rejects Once", func(t *testing.T) {
		err1 := errors.New("mock 1")
		err2 := errors.New("mock 2")

		p := New(func(resolve func(int), reject func(error)) {
			reject(err1)
			reject(err2)
		})

		if !assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			return
		}

		if !assertErr(t, err1, p) {
			return
		}

		assert.Never(t, func() bool {
			return !errEqual(err1, p)
		}, 50*time.Millisecond, time.Millisecond)
	})

	t.Run("Not Rejected After Resolve", func(t *testing.T) {
		err := errors.New("mock 1")

		p := New(func(resolve func(int), reject func(error)) {
			resolve(1)
			reject(err)
		})

		if !assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			return
		}

		if !assertVal(t, 1, p) || !assertNoErr(t, p) {
			return
		}

		assert.Never(t, func() bool {
			return !errEqual(nil, p)
		}, 50*time.Millisecond, time.Millisecond)
	})

	t.Run("Not Resolved After Reject", func(t *testing.T) {
		err := errors.New("mock 1")

		p := New(func(resolve func(int), reject func(error)) {
			reject(err)
			resolve(1)
		})

		if !assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			return
		}

		if !assertErr(t, err, p) || !assertNoVal(t, p) {
			return
		}

		assert.Never(t, func() bool {
			return !valEqual(0, p)
		}, 50*time.Millisecond, time.Millisecond)
	})
}

func TestWithResolvers(t *testing.T) {
	t.Run("Resolve", func(t *testing.T) {
		p, resolve, _ := WithResolvers[int]()
		resolve(1)

		if assertDone(t, p) {
			assertVal(t, 1, p)
			assertNoErr(t, p)
		}
	})

	t.Run("Reject", func(t *testing.T) {
		err := errors.New("mock")
		p, _, reject := WithResolvers[any]()
		reject(err)

		if assertDone(t, p) {
			assertNoVal(t, p)
			assertErr(t, err, p)
		}
	})
}

func TestAll(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		p := All[any]()

		if assertDone(t, p) {
			assertNoVal(t, p)
			assertNoErr(t, p)
		}
	})

	t.Run("All Resolved", func(t *testing.T) {
		p := All[int](Resolve(1), Resolve(2))

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, []int{1, 2}, p)
			assertNoErr(t, p)
		}
	})

	t.Run("At Least One Rejected", func(t *testing.T) {
		err := errors.New("mock")

		p := All[int](Resolve(1), Reject[int](err))

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertErr(t, err, p)
			assertNoVal(t, p)
		}
	})

	t.Run("Rejects When First Reject Seen", func(t *testing.T) {
		err := errors.New("mock")
		p1, _, _ := WithResolvers[int]()
		p2, _, reject2 := WithResolvers[int]()

		p := All[int](p1, p2)

		reject2(err)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertErr(t, err, p)
			assertNoVal(t, p)
		}
	})
}

func TestAllSettled(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		p := AllSettled[any]()

		if assertDone(t, p) {
			assertNoVal(t, p)
			assertNoErr(t, p)
		}
	})

	t.Run("Returns All States", func(t *testing.T) {
		p := AllSettled(Resolve(1), Resolve(2), Reject[int](errors.New("mock")))

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, []State{Fulfilled, Fulfilled, Rejected}, p)
			assertNoErr(t, p)
		}
	})
}

func TestRace(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		p := Race[any]()

		if assertDone(t, p) {
			assertNoVal(t, p)
			assertErr(t, ErrNoRacers, p)
		}
	})

	t.Run("Resolves With First Resolved", func(t *testing.T) {
		p1, _, _ := WithResolvers[int]()
		p2, resolve2, _ := WithResolvers[int]()

		p := Race(p1, p2)

		resolve2(2)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 2, p)
			assertNoErr(t, p)
		}
	})

	t.Run("Rejects With First Rejected", func(t *testing.T) {
		err := errors.New("mock")
		p1, _, _ := WithResolvers[int]()
		p2, _, reject2 := WithResolvers[int]()

		p := Race(p1, p2)

		reject2(err)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertErr(t, err, p)
			assertNoVal(t, p)
		}
	})
}

func TestAny(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		p := Any[any]()

		if assertDone(t, p) {
			assertNoVal(t, p)
			assertErr(t, ErrAllPromisesRejected, p)
		}
	})

	t.Run("Resolves With First Resolved", func(t *testing.T) {
		err := errors.New("mock")
		p1, _, reject1 := WithResolvers[int]()
		p2, resolve2, _ := WithResolvers[int]()

		p := Any(p1, p2)

		reject1(err)

		assert.Never(t, func() bool {
			return isDone(p)
		}, 50*time.Millisecond, time.Millisecond)

		resolve2(2)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 2, p)
			assertNoErr(t, p)
		}
	})

	t.Run("Rejects When All Rejected", func(t *testing.T) {
		err := errors.New("mock")
		p1, _, reject1 := WithResolvers[any]()
		p2, _, reject2 := WithResolvers[any]()

		p := Any(p1, p2)

		reject1(err)
		reject2(err)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertErr(t, ErrAllPromisesRejected, p)
			assertNoVal(t, p)
		}
	})
}

func TestResolve(t *testing.T) {
	p := Resolve(1)

	if assertDone(t, p) {
		assertVal(t, 1, p)
		assertNoErr(t, p)
	}
}

func TestReject(t *testing.T) {
	t.Run("With Error", func(t *testing.T) {
		err := errors.New("mock")
		p := Reject[any](err)

		if assertDone(t, p) {
			assertNoVal(t, p)
			assertErr(t, err, p)
		}
	})

	t.Run("Without Error", func(t *testing.T) {
		p := Reject[any](nil)

		if assertDone(t, p) {
			assertNoVal(t, p)
			assertErr(t, ErrRejected, p)
		}
	})
}

func TestDiverge(t *testing.T) {
	mockErr := errors.New("mock")

	tests := []struct {
		name    string
		promise *Promise[any]
		wantVal any
		wantErr error
	}{
		{name: "From Resolved", promise: Resolve[any](1), wantVal: 1, wantErr: nil},
		{name: "From Rejected", promise: Reject[any](mockErr), wantVal: nil, wantErr: mockErr},
	}

	t.Run("Resolve", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p := Diverge(tt.promise, func(v any, err error) (string, error) {
					assert.Equal(t, tt.wantVal, v)
					assert.Equal(t, tt.wantErr, err)
					return "1", nil
				})

				if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
					assertVal(t, "1", p)
					assertNoErr(t, p)
				}
			})
		}
	})

	t.Run("Reject", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockErrInner := errors.New("mock error inner")

				p := Diverge(tt.promise, func(v any, err error) (string, error) {
					assert.Equal(t, tt.wantVal, v)
					assert.Equal(t, tt.wantErr, err)
					return "", mockErrInner
				})

				if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
					assertNoVal(t, p)
					assertErr(t, mockErrInner, p)
				}
			})
		}
	})
}

func TestPromiseThen(t *testing.T) {
	t.Run("From Resolved", func(t *testing.T) {
		p := Resolve(1)
		seen := make(chan struct{})

		pt := p.Then(func(i int) *Promise[int] {
			assert.Equal(t, 1, i)
			close(seen)

			return nil
		})

		assert.Eventually(t, func() bool {
			select {
			case <-seen:
				return true
			default:
				return false
			}
		}, 50*time.Millisecond, time.Millisecond)

		if assert.Eventually(t, func() bool { return isDone(pt) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 1, pt)
			assertNoErr(t, pt)
		}
	})

	t.Run("From Rejected", func(t *testing.T) {
		err := errors.New("mock")
		p := Reject[any](err)
		seen := make(chan struct{})

		pt := p.Then(func(any) *Promise[any] {
			close(seen)

			return nil
		})

		assert.Never(t, func() bool {
			select {
			case <-seen:
				return true
			default:
				return false
			}
		}, 50*time.Millisecond, time.Millisecond)

		if assert.Eventually(t, func() bool { return isDone(pt) }, 50*time.Millisecond, time.Millisecond) {
			assertNoVal(t, pt)
			assertErr(t, err, pt)
		}
	})

	t.Run("Chain Resolve", func(t *testing.T) {
		p1 := Resolve(1)
		p2 := p1.Then(func(i int) *Promise[int] {
			assert.Equal(t, 1, i)
			return Resolve(i + 1)
		})

		if assert.Eventually(t, func() bool { return isDone(p2) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 2, p2)
			assertNoErr(t, p2)
		}
	})

	t.Run("Chain Reject", func(t *testing.T) {
		err := errors.New("mock")
		p1 := Resolve(1)
		p2 := p1.Then(func(i int) *Promise[int] {
			assert.Equal(t, 1, i)
			return Reject[int](err)
		})

		if assert.Eventually(t, func() bool { return isDone(p2) }, 50*time.Millisecond, time.Millisecond) {
			assertErr(t, err, p2)
			assertNoVal(t, p2)
		}
	})

	t.Run("No Callback", func(t *testing.T) {
		p := Resolve(1).Then(nil)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 1, p)
			assertNoErr(t, p)
		}
	})
}

func TestPromiseCatch(t *testing.T) {
	t.Run("From Resolved", func(t *testing.T) {
		p := Resolve(1)
		seen := make(chan struct{})

		pc := p.Catch(func(err error) *Promise[int] {
			close(seen)
			return nil
		})

		assert.Never(t, func() bool {
			select {
			case <-seen:
				return true
			default:
				return false
			}
		}, 50*time.Millisecond, time.Millisecond)

		if assert.Eventually(t, func() bool { return isDone(pc) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 1, pc)
			assertNoErr(t, pc)
		}
	})

	t.Run("From Rejected", func(t *testing.T) {
		mockErr := errors.New("mock")
		p := Reject[any](mockErr)
		seen := make(chan struct{})

		pc := p.Catch(func(err error) *Promise[any] {
			assert.Equal(t, mockErr, err)
			close(seen)
			return nil
		})

		assert.Eventually(t, func() bool {
			select {
			case <-seen:
				return true
			default:
				return false
			}
		}, 50*time.Millisecond, time.Millisecond)

		if assert.Eventually(t, func() bool { return isDone(pc) }, 50*time.Millisecond, time.Millisecond) {
			assertNoVal(t, pc)
			assertErr(t, mockErr, pc)
		}
	})

	t.Run("Chain Resolve", func(t *testing.T) {
		mockErr1 := errors.New("mock1")
		p1 := Reject[int](mockErr1)
		p2 := p1.Catch(func(err error) *Promise[int] {
			return Resolve(-1)
		})

		if assert.Eventually(t, func() bool { return isDone(p2) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, -1, p2)
			assertNoErr(t, p2)
		}
	})

	t.Run("Chain Reject", func(t *testing.T) {
		mockErr1 := errors.New("mock1")
		mockErr2 := errors.New("mock2")
		p1 := Reject[int](mockErr1)
		p2 := p1.Catch(func(err error) *Promise[int] {
			assert.Equal(t, mockErr1, err)
			return Reject[int](mockErr2)
		})

		if assert.Eventually(t, func() bool { return isDone(p2) }, 50*time.Millisecond, time.Millisecond) {
			assertNoVal(t, p2)
			assertErr(t, mockErr2, p2)
		}
	})

	t.Run("No Callback", func(t *testing.T) {
		err := errors.New("mock")
		p := Reject[any](err).Catch(nil)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertNoVal(t, p)
			assertErr(t, err, p)
		}
	})
}

func TestPromiseThenCatch(t *testing.T) {
	t.Run("From Resolved", func(t *testing.T) {
		p := Resolve(1)
		seenThen := make(chan struct{})
		seenCatch := make(chan struct{})

		ptc := p.ThenCatch(
			func(i int) *Promise[int] {
				assert.Equal(t, 1, i)
				close(seenThen)
				return nil
			},
			func(err error) *Promise[int] {
				close(seenCatch)
				return nil
			},
		)

		assert.Never(t, func() bool {
			select {
			case <-seenCatch:
				return true
			default:
				return false
			}
		}, 50*time.Millisecond, time.Millisecond)

		assert.Eventually(t, func() bool {
			select {
			case <-seenThen:
				return true
			default:
				return false
			}
		}, 50*time.Millisecond, time.Millisecond)

		if assert.Eventually(t, func() bool { return isDone(ptc) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 1, ptc)
			assertNoErr(t, ptc)
		}
	})

	t.Run("From Rejected", func(t *testing.T) {
		mockErr := errors.New("mock")
		p := Reject[any](mockErr)
		seenThen := make(chan struct{})
		seenCatch := make(chan struct{})

		ptc := p.ThenCatch(
			func(any) *Promise[any] {
				close(seenThen)
				return nil
			},
			func(err error) *Promise[any] {
				assert.Equal(t, mockErr, err)
				close(seenCatch)
				return nil
			},
		)

		assert.Never(t, func() bool {
			select {
			case <-seenThen:
				return true
			default:
				return false
			}
		}, 50*time.Millisecond, time.Millisecond)

		assert.Eventually(t, func() bool {
			select {
			case <-seenCatch:
				return true
			default:
				return false
			}
		}, 50*time.Millisecond, time.Millisecond)

		if assert.Eventually(t, func() bool { return isDone(ptc) }, 50*time.Millisecond, time.Millisecond) {
			assertNoVal(t, ptc)
			assertErr(t, mockErr, ptc)
		}
	})

	t.Run("Chain Resolve", func(t *testing.T) {
		p1 := Resolve(1)
		p2 := p1.ThenCatch(
			func(i int) *Promise[int] {
				assert.Equal(t, 1, i)
				return Resolve(i + 1)
			},
			func(err error) *Promise[int] {
				return Resolve(-1)
			},
		)

		if assert.Eventually(t, func() bool { return isDone(p2) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 2, p2)
			assertNoErr(t, p2)
		}
	})

	t.Run("Chain Reject", func(t *testing.T) {
		mockErr := errors.New("mock")
		p1 := Reject[int](mockErr)
		p2 := p1.ThenCatch(
			func(i int) *Promise[int] {
				return Resolve(i + 1)
			},
			func(err error) *Promise[int] {
				assert.Equal(t, mockErr, err)
				return Resolve(-1)
			},
		)

		if assert.Eventually(t, func() bool { return isDone(p2) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, -1, p2)
			assertNoErr(t, p2)
		}
	})

	t.Run("No Callback Resolve", func(t *testing.T) {
		p := Resolve(1).ThenCatch(nil, nil)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertVal(t, 1, p)
			assertNoErr(t, p)
		}
	})

	t.Run("No Callback Reject", func(t *testing.T) {
		err := errors.New("mock")
		p := Reject[any](err).ThenCatch(nil, nil)

		if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
			assertNoVal(t, p)
			assertErr(t, err, p)
		}
	})
}

func TestPromiseFinally(t *testing.T) {
	mockErr := errors.New("mock")

	tests := []struct {
		name    string
		promise *Promise[any]
		wantVal any
		wantErr error
	}{
		{name: "From Resolved", promise: Resolve[any](1), wantVal: 1, wantErr: nil},
		{name: "From Rejected", promise: Reject[any](mockErr), wantVal: nil, wantErr: mockErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(chan struct{})

			p := tt.promise.Finally(func() *Promise[any] {
				close(seen)
				return nil
			})

			assert.Eventually(t, func() bool {
				select {
				case <-seen:
					return true
				default:
					return false
				}
			}, 50*time.Millisecond, time.Millisecond)

			if assert.Eventually(t, func() bool { return isDone(p) }, 50*time.Millisecond, time.Millisecond) {
				assertVal(t, tt.wantVal, p)
				assertErr(t, tt.wantErr, p)
			}
		})
	}

	t.Run("Chain", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p2 := tt.promise.Finally(func() *Promise[any] {
					return Resolve(any(-1))
				})

				if assert.Eventually(t, func() bool { return isDone(p2) }, 50*time.Millisecond, time.Millisecond) {
					assertVal(t, -1, p2)
					assertNoErr(t, p2)
				}
			})
		}
	})

	t.Run("No Callback", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p2 := tt.promise.Finally(nil)

				if assert.Eventually(t, func() bool { return isDone(p2) }, 50*time.Millisecond, time.Millisecond) {
					assertVal(t, tt.wantVal, p2)
					assertErr(t, tt.wantErr, p2)
				}
			})
		}
	})
}

func TestPromiseAwait(t *testing.T) {
	t.Run("Resolved", func(t *testing.T) {
		p := New(func(resolve func(int), reject func(error)) {
			time.Sleep(20 * time.Microsecond)
			resolve(1)
		})

		v, err := p.Await()

		assert.Equal(t, 1, v)
		assert.NoError(t, err)
	})

	t.Run("Rejected", func(t *testing.T) {
		mockErr := errors.New("mock")
		p := New(func(resolve func(any), reject func(error)) {
			time.Sleep(20 * time.Microsecond)
			reject(mockErr)
		})

		v, err := p.Await()

		assert.Empty(t, v)
		assert.Equal(t, mockErr, err)
	})
}

func TestPromiseSettled(t *testing.T) {
	p, resolve, _ := WithResolvers[any]()
	isSettled := func() bool {
		select {
		case <-p.Settled():
			return true
		default:
			return false
		}
	}

	assert.False(t, isSettled())

	resolve("done")

	assert.True(t, isSettled())
}

func TestPromiseState(t *testing.T) {
	t.Run("Pending", func(t *testing.T) {
		p, _, _ := WithResolvers[any]()

		assert.Equal(t, Pending, p.State())
	})

	t.Run("Fulfilled", func(t *testing.T) {
		p, resolve, _ := WithResolvers[any]()

		resolve("done")

		assert.Equal(t, Fulfilled, p.State())
	})

	t.Run("Rejected", func(t *testing.T) {
		p, _, reject := WithResolvers[any]()

		reject(nil)

		assert.Equal(t, Rejected, p.State())
	})
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{state: Pending, want: "pending"},
		{state: Fulfilled, want: "fulfilled"},
		{state: Rejected, want: "rejected"},
		{state: 3, want: "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.String())
		})
	}
}

type stringerImpl struct {
	val string
}

func (s stringerImpl) String() string {
	return s.val
}

func getErr[T any](p *Promise[T]) error {
	return p.err
}
func getVal[T any](p *Promise[T]) T {
	return p.val
}

type tHelper interface {
	Helper()
}

func assertVal[T any](t assert.TestingT, val T, p *Promise[T]) bool {
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	return assert.Equal(t, val, getVal(p))
}

func assertNoVal[T any](t assert.TestingT, p *Promise[T]) bool {
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	var v T

	return assert.Equal(t, v, getVal(p))
}

func valEqual[T any](val T, p *Promise[T]) bool {
	return reflect.DeepEqual(val, getVal(p))
}

func assertErr[T any](t assert.TestingT, err error, p *Promise[T]) bool {
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	return assert.Equal(t, err, getErr(p))
}

func assertNoErr[T any](t assert.TestingT, p *Promise[T]) bool {
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	return assert.NoError(t, getErr(p))
}

func errEqual[T any](err error, p *Promise[T]) bool {
	return reflect.DeepEqual(err, getErr(p))
}

func assertDone[T any](t assert.TestingT, p *Promise[T]) bool {
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	if !isDone(p) {
		return assert.Fail(t, "Promise should be finished")
	}

	return true
}

func isDone[T any](p *Promise[T]) bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}
