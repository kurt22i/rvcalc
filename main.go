package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var simspertest = 100000      //iterations to run gcsim at when testing dps gain from upgrades.
var godatafile = "GOdata.txt" //filename of the GO data that will be used for weapons, current artifacts, and optimization settings besides ER. When go adds ability to optimize for x*output1 + y*output2, the reference sim will be used to determine optimization target.
var wantfile = "arti.csv"
var artis []Artifact
var wantdb []Want

func main() {
	flag.IntVar(&simspertest, "i", 10000, "sim iterations per test")
	flag.Parse()

	readArtifacts()
	readWant()
	evalartis()
	printResults()
}

func readWant() {

	f, err := os.ReadFile(wantfile)
	if err != nil {
		fmt.Printf("%v", err)
	}
	rawwant := string(f)
	wants := strings.Split(rawwant, "\n")
	for i := range wants {
		data := strings.Split(wants[i], ",")
		var w Want
		w.Char = data[0]
		sets := strings.Split(data[1], " ")
		w.Set = []string{sets[0]}
		for j := range sets {
			if j != 0 {
				w.Set = append(w.Set, sets[j])
			}
		}
		w.Mainstats = [][]float64{makestats("hpf", 1.0), makestats("atkf", 1.0), makestats(data[2], 1.0), makestats(data[3], 1.0), makestats(data[4], 1.0)}
		w.Substats = addsubs(newsubs(), makestats(data[5], 1.0))
		w.Substats = addsubs(w.Substats, makestats(data[6], 0.5))
	}
}

func makestats(stats string, val float64) []float64 {
	s := newsubs()
	if stats == "" {
		return s
	}
	sssss := strings.Split(stats, " ")
	for i := range sssss {
		s[getMeStat(sssss[i])] = val
	}
	return s
}

type Want struct {
	Set       []string
	Substats  []float64
	Char      string
	Mainstats [][]float64
}

type Artifact struct {
	Set      string
	Substats []float64
	Lines    int
	Mainstat int
	Level    int
	Slot     int
	BestOn   int
	RVon     int
	BestOff  int
	RVoff    int
	Rarity   int
}

func getSetID(dom string) int { //returns the internal id for an artifact
	id := -1
	for i, a := range artinames {
		if dom == a {
			id = i
		}
	}

	if id == -1 {
		fmt.Printf("no set found for %v", dom)
		return -1
	}

	return id
}

func printResults() {
	for _, a := range artis {
		name := artiname(a) + ":"
		on := "On: " + fmt.Sprintf("%.0f", a.RVon) + "%% for " + wantdb[a.BestOn].Char
		off := "Off: " + fmt.Sprintf("%.0f", a.RVoff) + "%% for " + wantdb[a.BestOff].Char
		fmt.Printf("%-40v%-40v%-40v\n", name, on, off)
	}
}

func artiname(a Artifact) string {
	name := a.Set
	name += "+" + strconv.Itoa(a.Level) + slotKey[a.Slot][:1]
	if a.Slot >= 2 {
		name += meStats[a.Mainstat]
	}
	name += "-"
	first := true
	for i, s := range a.Substats {
		if s > 0 {
			if !first {
				name += ","
			} else {
				first = false
			}
			name += strings.Replace(meStats[i], "f", "", 1) + fmt.Sprintf("%0.1f", s*float64(ispct[i]))
			if ispct[i] == 100 {
				name += "%"
			}
		}
	}
	return name
}

type GOarti struct {
	SetKey      string `json:"setKey"`
	Rarity      int    `json:"rarity"`
	Level       int    `json:"level"`
	SlotKey     string `json:"slotKey"`
	MainStatKey string `json:"mainStatKey"`
	Substats    []struct {
		Key   string  `json:"key"`
		Value float64 `json:"value"`
	} `json:"substats"`
	Location string `json:"location"`
	Exclude  bool   `json:"exclude"`
	Lock     bool   `json:"lock"`
}

func readArtifacts() {
	f, err := os.ReadFile(godatafile)
	if err != nil {
		fmt.Printf("%v", err)
	}
	rawgood := string(f)
	artisection := "[" + rawgood[strings.Index(rawgood, "artifacts\"")+12:strings.Index(rawgood, "weapons\"")-2]

	var gartis []GOarti
	err = json.Unmarshal([]byte(artisection), &artis)
	//asnowman := subsubs(ar)
	for i := range gartis { //this currently works by looking for an arti with 3 stats = and 1 stat bigger (main stat), should be good enough?
		var art Artifact
		art.Set = artiabbrs[getSetID(gartis[i].SetKey)]
		art.Lines = 0
		art.Mainstat = getStatID(gartis[i].MainStatKey)
		art.Level = gartis[i].Level
		art.Rarity = gartis[i].Rarity
		art.Slot = getSlotID(gartis[i].SlotKey)
		art.Substats = newsubs()
		art.BestOn = -1
		art.BestOff = -1
		art.RVon = -1
		art.RVoff = -1
		for _, s := range gartis[i].Substats {
			if s.Key == "" {
				break
			}
			art.Substats[getStatID(s.Key)] += s.Value / float64(ispct[getStatID(s.Key)])
			art.Lines++
		}
		artis = append(artis, art)
	}
}

func evalartis() {
	for i, w := range wantdb {
		for _, a := range artis {
			rv := maxrv(a, w) + currv(a, w)
			on := isOn(a, w)
			if on {
				if rv > a.RVon {
					a.RVon = rv
					a.BestOn = i
				}
			} else {
				if rv > a.RVoff {
					a.RVoff = rv
					a.BestOff = i
				}
			}
		}
	}
}

func isOn(a Artifact, w Want) bool {
	for _, s := range w.Set {
		if s == a.Set {
			return true
		}
	}
	return false
}

func maxrv(a Artifact, w Want) int {
	ptrolls := 5 - a.Level/4
	rv := 0
	if a.Lines == 3 {
		ptrolls--
		//choose the best stat not currently on arti
		w2 := w.Substats
		for i := range a.Substats {
			if a.Substats[i] > 0 {
				w2[i] = 0
			}
		}
		rv += int(100.0 * maxsub(w2))
	}
	w2 := w.Substats
	if a.Lines == 4 { //if 4 lines, best stat to upgrade might not be the BIS one
		for i := range a.Substats {
			if a.Substats[i] == 0 {
				w2[i] = 0
			}
		}
	}
	rv += ptrolls * int(100.0*maxsub(w2))
	return int(float64(rv)*w.Mainstats[a.Slot][a.Mainstat]) + currv(a, w)
}

func currv(a Artifact, w Want) int {
	rv := 0
	for i := range a.Substats {
		rv += int(a.Substats[i] / maxrolls[i] * w.Substats[i] * 100.0)
	}
	return int(float64(rv) * w.Mainstats[a.Slot][a.Mainstat])
}

func maxsub(subs []float64) float64 {
	max := -1.0
	for _, s := range subs {
		if s > max {
			max = s
		}
	}
	return max
}

func newsubs() []float64 { //empty stat array
	return []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
}

func AGstatid(key string, ispt bool) int {
	for i, k := range AGstatKeys {
		if k == key {
			if i < 6 && ispt {
				return i + 1 //the key for flat vs % hp, atk and def is the same, so we have to look at the value
			}
			return i
		}
	}
	fmt.Printf("no stat found for the AG key %v", key)
	return -1
}

func getStatID(key string) int {
	for i, k := range statKey {
		if k == key {
			return i
		}
	}
	fmt.Printf("%v not recognized as a key", key)
	return -1
}

func getMeStat(key string) int {
	for i, k := range meStats {
		if k == key {
			return i
		}
	}
	fmt.Printf("%v not recognized as a mestat", key)
	return -1
}

func getSlotID(key string) int {
	for i, k := range slotKey {
		if k == key {
			return i
		}
	}
	fmt.Printf("%v not recognized as a key", key)
	return -1
}

//ugly sorting code - sorts sim chars by dps, which is the order we should optimize them in (except this is a waste bc user has to specify anyway rn)
/*chars := []string{"", "", "", ""}
chardps := []float64{-1.0, -1.0, -1.0, -1.0}
for i := range baseline.CharDPS {
	chardps[i] = baseline.CharDPS[i].DPS1.Mean
}
sort.Float64s(chardps)
for i := range baseline.Characters {
	for j := range chardps {
		if baseline.CharDPS[i].DPS1.Mean == chardps[j] {
			chars[j] = baseline.Characters[i].Name
		}
	}
}*/

var artinames = []string{"BlizzardStrayer", "HeartOfDepth", "ViridescentVenerer", "MaidenBeloved", "TenacityOfTheMillelith", "PaleFlame", "HuskOfOpulentDreams", "OceanHuedClam", "ThunderingFury", "Thundersoother", "EmblemOfSeveredFate", "ShimenawasReminiscence", "NoblesseOblige", "BloodstainedChivalry", "CrimsonWitchOfFlames", "Lavawalker"}
var artiabbrs = []string{"bs", "hod", "vv", "mb", "tom", "pf", "husk", "ohc", "tf", "ts", "esf", "sr", "no", "bsc", "cw", "lw"}

var simChars = []string{"ganyu", "rosaria", "kokomi", "venti", "ayaka", "mona", "albedo", "fischl", "zhongli", "raiden", "bennett", "xiangling", "xingqiu", "shenhe", "yae", "kazuha", "beidou", "sucrose", "jean", "chongyun", "yanfei", "keqing", "tartaglia", "eula", "lisa", "yunjin"}
var simCharsID = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}
var GOchars = []string{"Ganyu", "Rosaria", "SangonomiyaKokomi", "Venti", "KamisatoAyaka", "Mona", "Albedo", "Fischl", "Zhongli", "RaidenShogun", "Bennett", "Xiangling", "Xingqiu", "Shenhe", "YaeMiko", "KaedeharaKazuha", "Beidou", "Sucrose", "Jean", "Chongyun", "Yanfei", "Keqing", "Tartaglia", "Eula", "Lisa", "YunJin"}

var slotKey = []string{"flower", "plume", "sands", "goblet", "circlet"}
var statKey = []string{"atk", "atk_", "hp", "hp_", "def", "def_", "eleMas", "enerRech_", "critRate_", "critDMG_", "heal_", "pyro_dmg_", "electro_dmg_", "cryo_dmg_", "hydro_dmg_", "anemo_dmg_", "geo_dmg_", "physical_dmg_"}
var meStats = []string{"atkf", "atk", "hpf", "hp", "deff", "def", "em", "er", "cr", "cd", "heal", "pyro", "electro", "cryo", "hydro", "anemo", "geo", "phys"}
var AGstatKeys = []string{"Atk", "n/a", "hp", "n/a", "Def", "n/a", "ele_mas", "EnergyRecharge", "CritRate", "CritDMG", "HealingBonus", "pyro", "electro", "cryo", "hydro", "anemo", "geo", "physicalDmgBonus"}
var ispct = []int{1, 100, 1, 100, 1, 100, 1, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}

var maxrolls = []float64{19.45, 0.0583, 298.75, 0.0583, 23.15, 0.0729, 23.31, 0.0648, 0.0389, 0.0777, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0, -1.0}

func addsubs(s1, s2 []float64) []float64 {
	add := newsubs()
	for i := range add {
		add[i] = s1[i] + s2[i]
	}
	return add
}

func subsubs(s1, s2 []float64) []float64 {
	sub := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := range sub {
		sub[i] = s1[i] - s2[i] //math.Max(0, s1[i]-s2[i])
	}
	return sub
}

func multsubs(s []float64, mult float64) []float64 {
	sub := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := range sub {
		sub[i] = s[i] * mult
	}
	return sub
}

var subchance = []int{6, 4, 6, 4, 6, 4, 4, 4, 3, 3}
var srolls = []float64{0.824, 0.941, 1.059, 1.176}

var rollints = []int{1, 1, 30, 80, 50}
var mschance = [][]int{ //chance of mainstat based on arti type
	{0, 0, 1},
	{1},
	{0, 8, 0, 8, 0, 8, 3, 3},
	{0, 17, 0, 17, 0, 16, 2, 0, 0, 0, 0, 4, 4, 4, 4, 4, 4, 4},
	{0, 11, 0, 11, 0, 11, 2, 0, 5, 5, 5},
}
