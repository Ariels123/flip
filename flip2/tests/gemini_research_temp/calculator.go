package calculator

import (
	"errors"
	"math"
)

// Add returns the sum of two numbers.
func Add(a, b float64) float64 {
	return a + b
}

// Subtract returns the difference between two numbers.
func Subtract(a, b float64) float64 {
	return a - b
}

// Multiply returns the product of two numbers.
func Multiply(a, b float64) float64 {
	return a * b
}

// Divide returns the quotient of two numbers.
// It returns an error if the divisor is zero.
func Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("cannot divide by zero")
	}
	// Check for overflow/infinity
	if math.IsInf(a/b, 0) {
		return 0, errors.New("result is infinity")
	}
	return a / b, nil
}

// Power returns the base raised to the exponent.
func Power(base, exponent float64) float64 {
	return math.Pow(base, exponent)
}

// Sqrt returns the square root of a number.
// It returns an error if the number is negative.
func Sqrt(n float64) (float64, error) {
	if n < 0 {
		return 0, errors.New("cannot calculate square root of a negative number")
	}
	return math.Sqrt(n), nil
}

// Modulus returns the remainder of the division of a by b.
func Modulus(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("cannot calculate modulus with zero divisor")
	}
	return math.Mod(a, b), nil
}
