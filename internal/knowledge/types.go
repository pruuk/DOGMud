package knowledge

type SubjectType string

const (
	SubjectPlayer SubjectType = "player"
	SubjectMob    SubjectType = "mob"
)

type Subject struct {
	Type SubjectType `yaml:"type"`
	Id   int         `yaml:"id"`
}

func PlayerSubject(userId int) Subject { return Subject{Type: SubjectPlayer, Id: userId} }
func MobSubject(mobId int) Subject     { return Subject{Type: SubjectMob, Id: mobId} }

type Source string

const (
	SourceWitnessed Source = "witnessed"
	SourceTold      Source = "told"
	SourceDeduced   Source = "deduced"
	SourceUnknown   Source = "unknown"
)

type Confidence string

const (
	ConfidenceHigh Confidence = "high"
	ConfidenceMed  Confidence = "med"
	ConfidenceLow  Confidence = "low"
)

type Observation struct {
	Room  int    `yaml:"room"`
	Round uint64 `yaml:"round"`
}

type Record struct {
	Subject          Subject       `yaml:"subject"`
	HasMet           bool          `yaml:"has_met"`
	NameLearned      string        `yaml:"name_learned,omitempty"`
	Source           Source        `yaml:"source"`
	Confidence       Confidence    `yaml:"confidence"`
	LastSeenRoom     int           `yaml:"last_seen_room"`
	LastSeenRound    uint64        `yaml:"last_seen_round"`
	Observations     []Observation `yaml:"observations,omitempty"`
	CrimesWitnessed  []int         `yaml:"crimes_witnessed,omitempty"`
	LearnedRound     uint64        `yaml:"learned_round"`
	LastUpdatedRound uint64        `yaml:"last_updated_round"`
}

type ObserverFile struct {
	ObserverMobId int       `yaml:"observer_mob_id"`
	ObserverName  string    `yaml:"observer_name"`
	Records       []*Record `yaml:"records"`

	// Write-behind bookkeeping (see persistence.go). Unexported and untagged,
	// so YAML never reads or writes them. All three are guarded by
	// knowledgeCacheMu. gen counts mutations; savedGen is the gen whose bytes
	// are confirmed on disk; preparedGen is the gen of the latest snapshot
	// handed to the autosave queue. The file is dirty while gen != savedGen.
	gen         uint64
	savedGen    uint64
	preparedGen uint64
}
