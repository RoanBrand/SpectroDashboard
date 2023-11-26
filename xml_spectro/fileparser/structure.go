package fileparser

// Every file seems to contain only one sample result, but it may have more.
type sampleResultsXMLFile struct {
	SampleResults []SampleResult `xml:"SampleResult"`
}

type SampleResult struct {
	Timestamp string `xml:"RecalculationDateTime,attr"`
	Method    string `xml:"MethodName,attr"`

	SampleIDs             []SampleID             `xml:"SampleIDs>SampleID"`
	MeasurementReplicates []MeasurementReplicate `xml:"MeasurementReplicates>MeasurementReplicate"`
	MeasurementStatistics []Measurement          `xml:"MeasurementStatistics>Measurement"`
}

type SampleID struct {
	Name  string `xml:"IDName"`
	Value string `xml:"IDValue"`
}

type MeasurementReplicate struct {
	IsDeleted   string      `xml:"IsDeleted,attr"`
	Measurement Measurement `xml:"Measurement"`
}

type Measurement struct {
	CheckType string `xml:"CheckType,attr"`

	Lines    []Line    `xml:"Lines>Line"`
	Elements []Element `xml:"Elements>Element"`
}

type Line struct {
	Name string `xml:"LineName,attr"`
	Type string `xml:"Type,attr"`

	LineResults []Result `xml:"LineResult"`
}

type Element struct {
	Name string `xml:"ElementName,attr"`
	Type string `xml:"Type,attr"`

	ElementResults []Result `xml:"ElementResult"`
}

type Result struct {
	Type     string `xml:"Type,attr"`
	Kind     string `xml:"Kind,attr"`
	Unit     string `xml:"Unit,attr"`
	StatType string `xml:"StatType,attr"` // 'Reported' seems to be the one we want

	ResultValue float64 `xml:"ResultValue"`
}

// XML helper functions.
func (sr *SampleResult) findSampleId(id string) string {
	for _, sId := range sr.SampleIDs {
		if sId.Name == id {
			return sId.Value
		}
	}
	return ""
}

func (sr *SampleResult) SampleID() string {
	return sr.findSampleId("Sample ID")
}

func (sr *SampleResult) Furnace() string {
	return sr.findSampleId("Quality") // seems the operators enter into the wrong field
}

func (sr *SampleResult) Operator() string {
	return sr.findSampleId("Operator")
}

func (el *Element) reportedResult() *Result {
	for _, res := range el.ElementResults {
		if res.StatType == "Reported" {
			return &res
		}
	}
	return nil
}
