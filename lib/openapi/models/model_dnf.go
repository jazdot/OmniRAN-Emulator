package models

type Dnf struct {
	DnfUnits []DnfUnit `json:"dnfUnits" bson:"dnfUnits"`
}
