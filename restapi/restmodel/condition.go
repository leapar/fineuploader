package restmodel

type Operator string

const (
	OP_equal = "="
	OP_not_equal = "<>"
	OP_equal_little = "<="
	OP_big = ">"
	OP_big_equal = ">="
	OP_startsWith = "startsWith"
	OP_endsWith = "endsWith"
	OP_contains = "contains"
	OP_notEmpty = "notEmpty"
	OP_in = "in"
	OP_notIn = "notIn"
)

type Condition struct {
	Property string `json:"property,omitempty"`
	Operator Operator `json:"operator,omitempty"`
	Value string `json:"value,omitempty"`
}

type Group string
const (
	Group_OR = "OR"
	Group_AND = "AND"
)

type Conditions struct {
	Conditions []Condition `json:"conditions,omitempty"`
	Group  Group `json:"group,omitempty"`
}
