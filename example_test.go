package promise_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/artem328/go-promise"
)

func Example() {
	p := promise.New(func(resolve func(int), reject func(error)) {
		fmt.Println("entering p")
		time.Sleep(500 * time.Millisecond)
		fmt.Println("resolving from p")
		resolve(1)
	}).
		Then(func(i int) *promise.Promise[int] {
			fmt.Println("t1 value received:", i)
			p, resolve, _ := promise.WithResolvers[int]()

			go func() {
				time.Sleep(500 * time.Millisecond)
				fmt.Println("resolving from t1")
				resolve(i + 9)
			}()

			return p
		}).
		Then(func(i int) *promise.Promise[int] {
			fmt.Println("t2 value received:", i)
			fmt.Println("t2 resolving")
			return promise.Resolve(i * 2)
		}).
		Catch(func(err error) *promise.Promise[int] {
			fmt.Println("we failed somewhere in chain", err)
			return nil
		}).
		Finally(func() *promise.Promise[int] {
			fmt.Println("all done, let's see the result")
			return nil
		})

	fmt.Println(p.Await())

	// Output:
	// entering p
	// resolving from p
	// t1 value received: 1
	// resolving from t1
	// t2 value received: 10
	// t2 resolving
	// all done, let's see the result
	// 20 <nil>
}

func ExampleNew() {
	sayHello := func() (string, error) { return "hello", nil }

	p := promise.New(func(resolve func(string), reject func(error)) {
		v, err := sayHello()
		if err != nil {
			reject(err) // reject the promise p
			return
		}
		resolve(v) // fulfill the promise p
	})

	fmt.Println(p.Await())

	// Output: hello <nil>
}

func ExampleWithResolvers() {
	sayHello := func() (string, error) { return "hello", nil }
	p, resolve, reject := promise.WithResolvers[string]()

	go func() {
		v, err := sayHello()
		if err != nil {
			reject(err) // reject the promise p
			return
		}
		resolve(v) // fulfill the promise p
	}()

	fmt.Println(p.Await())

	// Output: hello <nil>
}

func ExampleAll_fulfilled() {
	p1 := promise.Resolve("p1")
	p2 := promise.Resolve("p2")

	p := promise.All(p1, p2)

	fmt.Println(p.Await())

	// Output: [p1 p2] <nil>
}

func ExampleAll_rejected() {
	p1 := promise.Resolve("p1")
	p2 := promise.Reject[string](errors.New("error"))

	p := promise.All(p1, p2)

	fmt.Println(p.Await())

	// Output: [] error
}

func ExampleAllSettled() {
	p1 := promise.Resolve("p1")
	p2 := promise.Reject[string](errors.New("error"))
	p3 := promise.Resolve("p3")

	p := promise.AllSettled(p1, p2, p3)

	fmt.Println(p.Await())

	// Output: [fulfilled rejected fulfilled] <nil>
}

func ExampleRace_fulfilled() {
	p1 := promise.New(func(resolve func(string), _ func(error)) {
		time.Sleep(500 * time.Millisecond)
		fmt.Println("resolving p1")
		resolve("p1")
	})
	p2 := promise.New(func(resolve func(string), _ func(error)) {
		fmt.Println("resolving p2")
		resolve("p2")
	})

	p := promise.Race(p1, p2)

	fmt.Println(p.Await())

	time.Sleep(time.Second)

	// Output:
	// resolving p2
	// p2 <nil>
	// resolving p1
}

func ExampleRace_rejected() {
	p1 := promise.New(func(resolve func(string), _ func(error)) {
		time.Sleep(500 * time.Millisecond)
		fmt.Println("resolving p1")
		resolve("p1")
	})
	p2 := promise.New(func(_ func(string), reject func(error)) {
		fmt.Println("rejecting p2")
		reject(errors.New("error"))
	})

	p := promise.Race(p1, p2)

	v, err := p.Await()
	fmt.Printf("%#v %v\n", v, err)

	time.Sleep(time.Second)

	// Output:
	// rejecting p2
	// "" error
	// resolving p1
}

func ExampleAny_fulfilled() {
	p1 := promise.Reject[string](errors.New("error1"))
	p2 := promise.Resolve("p2")

	p := promise.Any(p1, p2)

	fmt.Println(p.Await())

	// Output: p2 <nil>
}

func ExampleAny_rejected() {
	p1 := promise.Reject[any](errors.New("error1"))
	p2 := promise.Reject[any](errors.New("error2"))

	p := promise.Any(p1, p2)

	fmt.Println(p.Await())

	// Output: <nil> all promises rejected
}

func ExampleResolve() {
	p := promise.Resolve("hello")

	fmt.Println(p.Await())

	// Output: hello <nil>
}

func ExampleReject() {
	p := promise.Reject[int](errors.New("error"))

	fmt.Println(p.Await())

	// Output: 0 error
}

func ExamplePromise_Then() {
	p := promise.Resolve(1).
		Then(func(i int) *promise.Promise[int] {
			fmt.Println("in then:", i)
			return nil
		})

	fmt.Println(p.Await())

	// Output:
	// in then: 1
	// 1 <nil>
}

func ExamplePromise_Then_chain() {
	p := promise.Resolve(2).
		Then(func(i int) *promise.Promise[int] {
			fmt.Println("in then:", i)
			return promise.Resolve(i * 3)
		})

	fmt.Println(p.Await())

	// Output:
	// in then: 2
	// 6 <nil>
}

func ExamplePromise_Then_rejected() {
	p := promise.Reject[int](errors.New("error")).
		Then(func(i int) *promise.Promise[int] {
			fmt.Println("not reachable", i)
			return nil
		})

	fmt.Println(p.Await())

	// Output:
	// 0 error
}

func ExamplePromise_Catch() {
	p := promise.Reject[int](errors.New("error")).
		Catch(func(err error) *promise.Promise[int] {
			fmt.Println("caught", err)
			return nil
		})

	fmt.Println(p.Await())

	// Output:
	// caught error
	// 0 error
}

func ExamplePromise_Catch_chain() {
	p := promise.Reject[int](errors.New("error")).
		Catch(func(err error) *promise.Promise[int] {
			fmt.Println("caught", err)
			return promise.Resolve(1)
		})

	fmt.Println(p.Await())

	// Output:
	// caught error
	// 1 <nil>
}

func ExamplePromise_Catch_fulfilled() {
	p := promise.Resolve(1).
		Catch(func(err error) *promise.Promise[int] {
			fmt.Println("not reachable", err)
			return nil
		})

	fmt.Println(p.Await())

	// Output:
	// 1 <nil>
}

func ExamplePromise_Finally_fulfilled() {
	p := promise.Resolve(1).
		Finally(func() *promise.Promise[int] {
			fmt.Println("promise settled")
			return nil
		})

	fmt.Println(p.Await())

	// Output:
	// promise settled
	// 1 <nil>
}

func ExamplePromise_Finally_rejected() {
	p := promise.Reject[int](errors.New("error")).
		Finally(func() *promise.Promise[int] {
			fmt.Println("promise settled")
			return nil
		})

	fmt.Println(p.Await())

	// Output:
	// promise settled
	// 0 error
}

func ExamplePromise_Finally_chain() {
	p := promise.Resolve(1).
		Finally(func() *promise.Promise[int] {
			fmt.Println("promise settled")
			return promise.Resolve(2)
		})

	fmt.Println(p.Await())

	// Output:
	// promise settled
	// 2 <nil>
}
