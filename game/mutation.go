package game

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"
)

type Mutation struct {
	ID          string
	Name        string
	Description string
}

var Mutations = []Mutation{
	{ID: "no_skills", Name: "스킬 봉인", Description: "직업 스킬 사용 불가"},
	{ID: "speed_run", Name: "스피드 런", Description: "턴 제한 시간 절반"},
	{ID: "no_shop", Name: "상점 폐쇄", Description: "상점 이용 불가"},
	{ID: "glass_cannon", Name: "유리 대포", Description: "피해 2배, HP 절반"},
	{ID: "elite_flood", Name: "엘리트 범람", Description: "모든 몬스터가 엘리트"},
}

func GetWeeklyMutation() Mutation {
	year, week := time.Now().ISOWeek()
	h := sha256.Sum256([]byte(fmt.Sprintf("mutation:%d:%d", year, week)))
	idx := int(binary.BigEndian.Uint64(h[:8]) % uint64(len(Mutations)))
	return Mutations[idx]
}
