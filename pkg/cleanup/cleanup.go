// Package cleanup provides helpers to make it easy to do cleanups.
//
// For example, if a function involves a few separate sub-calls with
// their own cleanups, a typical code would have to do something like
// the following:
//
//     resource1, cleanup1, err := doFirst()
//     if err != nil { .... }
//
//     resource2, cleanup2, err := doSecond()
//     if err != nil {
//         defer cleanup1()
//         nil, nil, return err
//     }
//
//     resource3, cleanup3, err := doThird()
//     if err != nil {
//         defer cleanup1()
//         defer cleanup2()
//         nil, nil, return err
//     }
//     cleanupAll = func() {
//        defer cleanup1()
//        defer cleanup2()
//        cleanup3()
//     }
//     return combine(resource1, resource2, resource3), cleanupAll, nil
//
// The code above is both error prone and unsightly. With this package
// it would look like so:
//
//     var cleanup1, cleanup2, cleanup2 func()
//     cleanups := cleanups.Funcs{&cleanup1, &cleanup2, &cleanup3}
//     defer cleanups.Run()
//
//     resource1, cleanup1, err := doFirst()
//     if err != nil { return err }
//
//     resource2, cleanup2, err := doSecond()
//     if err != nil { return err }
//
//     resource3, cleanup3, err := doThird()
//     if err != nil { return err }
//
//     return combine(resource1, resource2, resource3), cleanups.All(), nil
package cleanup

// Funcs is an array of function pointers.  The actual pointers should
// not be nil though what they point to can be nil.
type Funcs []*func()

// Run executes all the functions in reverse order ensuring that even
// if one panics, the following functions are still called.
func (f *Funcs) Run() {
	for _, ff := range *f {
		if (*ff) != nil {
			defer (*ff)() //nolint:gocritic // Why: intentional usage of defer here
		}
	}
}

// All zeros the functions list. It captures the list before zeroing
// and returns Run.  Executing the returned function will effectively
// call all the entries that used to exist in f in reverse order.
func (f *Funcs) All() func() {
	before := *f
	*f = nil
	return before.Run
}
