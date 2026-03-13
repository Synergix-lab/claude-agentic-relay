package models

// Skill is a structured capability in the skill catalog.
type Skill struct {
	ID          string `json:"id"`
	Project     string `json:"project"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tags        string `json:"tags"` // JSON array
	CreatedAt   string `json:"created_at"`
}

// ProfileSkill links a profile to a skill with a proficiency level.
type ProfileSkill struct {
	ProfileID   string `json:"profile_id"`
	SkillID     string `json:"skill_id"`
	Proficiency string `json:"proficiency"` // capable, expert, learning
}
