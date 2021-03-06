package fakes

type TerraformExecutor struct {
	ApplyCall struct {
		CallCount int
		Receives  struct {
			Inputs   map[string]string
			Template string
			TFState  string
		}
		Returns struct {
			TFState string
			Error   error
		}
	}
	DestroyCall struct {
		CallCount int
		Receives  struct {
			Inputs   map[string]string
			Template string
			TFState  string
		}
		Returns struct {
			TFState string
			Error   error
		}
	}
	VersionCall struct {
		CallCount int
		Returns   struct {
			Version string
			Error   error
		}
	}
	OutputCall struct {
		Stub      func(string) (string, error)
		CallCount int
		Receives  struct {
			TFState    string
			OutputName string
		}
		Returns struct {
			Output string
			Error  error
		}
	}
	OutputsCall struct {
		Stub      func() (map[string]interface{}, error)
		CallCount int
		Receives  struct {
			TFState string
		}
		Returns struct {
			Outputs map[string]interface{}
			Error   error
		}
	}
}

func (t *TerraformExecutor) Apply(inputs map[string]string, template, tfState string) (string, error) {
	t.ApplyCall.CallCount++
	t.ApplyCall.Receives.Inputs = inputs
	t.ApplyCall.Receives.Template = template
	t.ApplyCall.Receives.TFState = tfState
	return t.ApplyCall.Returns.TFState, t.ApplyCall.Returns.Error
}

func (t *TerraformExecutor) Destroy(inputs map[string]string, template, tfState string) (string, error) {
	t.DestroyCall.CallCount++
	t.DestroyCall.Receives.Inputs = inputs
	t.DestroyCall.Receives.Template = template
	t.DestroyCall.Receives.TFState = tfState
	return t.DestroyCall.Returns.TFState, t.DestroyCall.Returns.Error
}

func (t *TerraformExecutor) Version() (string, error) {
	t.VersionCall.CallCount++
	return t.VersionCall.Returns.Version, t.VersionCall.Returns.Error
}

func (t *TerraformExecutor) Output(tfState, outputName string) (string, error) {
	t.OutputCall.CallCount++
	t.OutputCall.Receives.TFState = tfState
	t.OutputCall.Receives.OutputName = outputName

	if t.OutputCall.Stub != nil {
		return t.OutputCall.Stub(outputName)
	}

	return t.OutputCall.Returns.Output, t.OutputCall.Returns.Error
}

func (t *TerraformExecutor) Outputs(tfState string) (map[string]interface{}, error) {
	t.OutputsCall.CallCount++
	t.OutputsCall.Receives.TFState = tfState

	if t.OutputsCall.Stub != nil {
		return t.OutputsCall.Stub()
	}

	return t.OutputsCall.Returns.Outputs, t.OutputsCall.Returns.Error
}
