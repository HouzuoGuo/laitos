package testingstub

/*
T defined several functions that are satisfied by "testing.T".
Most daemons have their testing routine written in non-testing files so that the routines can be used from multiple
packages. However, "testing" package has a package initialiser that puts test mode flags into global flags, which is highly
unnecessary. Hence this interface is defined to avoid triggering the initialiser.
*/
type T interface {
	Helper()
	Error(...interface{})
	Errorf(string, ...interface{})
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Fail()
	FailNow()
	Failed() bool
	Log(...interface{})
	Logf(string, ...interface{})
	Skip(...interface{})
}
