// Package predict provides a set of helper routines for predicting

package predict

import (
	"errors"

	"github.com/gonum/matrix/mat64"

	"github.com/reggo/common"
)

type BatchPredictor interface {
	NewPredictor() Predictor // Returns a predictor. This exists so that methods can create temporary data if necessary
}

type Predictor interface {
	Predict(input, output []float64)
}

// TODO: Replace these errors with a better location for error checking

func BatchPredict(batch BatchPredictor, inputs mat64.Matrix, outputs mat64.Mutable, inputDim, outputDim int, grainSize int) (mat64.Mutable, error) {

	// TODO: Add in something about error

	// Check that the inputs and outputs are the right sizes
	nSamples, dimInputs := inputs.Dims()
	if inputDim != dimInputs {
		return outputs, errors.New("predict batch: input dimension mismatch")
	}

	if outputs == nil {
		outputs = mat64.NewDense(nSamples, outputDim, nil)
	} else {
		nOutputSamples, dimOutputs := outputs.Dims()
		if dimOutputs != outputDim {
			return outputs, errors.New("predict batch: output dimension mismatch")
		}
		if nSamples != nOutputSamples {
			return outputs, errors.New("predict batch: rows mismatch")
		}
	}

	// Perform predictions in parallel. For each parallel call, form a new predictor so that
	// memory allocations are saved and no race condition happens.

	// If the input and/or output is a RowViewer, save time by avoiding a copy
	inputRVer, inputIsRowViewer := inputs.(mat64.RowViewer)
	outputRVer, outputIsRowViewer := outputs.(mat64.RowViewer)

	var f func(start, end int)

	// wrapper function to allow parallel prediction. Uses RowView if the type has it
	switch {
	default:
		panic("Shouldn't be here")
	case inputIsRowViewer, outputIsRowViewer:
		f = func(start, end int) {
			p := batch.NewPredictor()
			for i := start; i < end; i++ {
				p.Predict(inputRVer.RowView(i), outputRVer.RowView(i))
			}
		}

	case inputIsRowViewer && !outputIsRowViewer:
		f = func(start, end int) {
			p := batch.NewPredictor()
			output := make([]float64, outputDim)
			for i := start; i < end; i++ {
				for j := range output {
					output[j] = outputs.At(i, j)
				}
				p.Predict(inputRVer.RowView(i), output)
				for j, out := range output {
					outputs.Set(i, j, out)
				}
			}
		}
	case !inputIsRowViewer && outputIsRowViewer:
		f = func(start, end int) {
			p := batch.NewPredictor()
			input := make([]float64, inputDim)
			for i := start; i < end; i++ {
				for j := range input {
					input[j] = inputs.At(i, j)
				}
				p.Predict(input, outputRVer.RowView(i))
			}
		}
	case !inputIsRowViewer && !outputIsRowViewer:
		f = func(start, end int) {
			p := batch.NewPredictor()
			input := make([]float64, inputDim)
			output := make([]float64, outputDim)
			for i := start; i < end; i++ {
				for j := range input {
					input[j] = inputs.At(i, j)
				}
				for j := range output {
					output[j] = outputs.At(i, j)
				}
				p.Predict(input, output)
				for j, out := range output {
					outputs.Set(i, j, out)
				}
			}
		}
	}

	common.ParallelFor(nSamples, grainSize, f)
	return outputs, nil
}
