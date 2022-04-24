package details

type Objective struct {
	Name   string `validate:"required"`
	Status int    `validate:"oneof=0 1 2"`
}
