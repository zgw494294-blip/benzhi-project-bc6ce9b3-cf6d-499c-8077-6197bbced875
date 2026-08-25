package audit

type CommandPayload struct {
	Operation string `json:"operation"`
	Revision  int64  `json:"revision"`
	ResultRef string `json:"resultRef"`
}
type ReviewPayload struct {
	ConflictID string `json:"conflictId"`
	Decision   string `json:"decision"`
	Comment    string `json:"comment"`
}
type PermitPayload struct {
	SerialNumber   string `json:"serialNumber"`
	BaselineDigest string `json:"baselineDigest"`
	PermitDigest   string `json:"permitDigest"`
}
