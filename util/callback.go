package util

// Callback is a callback interface to perform actions after processing is completed.
type Callback[T any] func(obj T)
