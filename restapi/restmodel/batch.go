package restmodel

type BatchColorboard struct {
	ColorboardPath *ExtFile `json:"colorboardPath,omitempty"`
	Name string `json:"name,omitempty"`
	Id string `json:"id,omitempty"`
	HardwarePath *ExtFile `json:"hardwarePath,omitempty"`
}
