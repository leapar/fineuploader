package restmodel

type ProjectModel struct {
	Id string `json:"id,omitempty"`
	OriginFile *ExtFile `json:"originFile,omitempty"`
	DeployFile *ExtFile `json:"deployFile,omitempty"`
}

type ProjectModelTaskPlan struct {
	Id string `json:"id,omitempty"`
	ExtFile *ExtFile `json:"extFile,omitempty"`
}