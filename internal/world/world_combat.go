package world

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/IYanHua/mdt-server/internal/vanilla"
)

type BulletEvent struct {
	Team      TeamID
	X         float32
	Y         float32
	Angle     float32
	Damage    float32
	BulletTyp int16
}

type simBullet struct {
	ID                   int32
	Team                 TeamID
	X                    float32
	Y                    float32
	VX                   float32
	VY                   float32
	Damage               float32
	SplashDamage         float32
	LifeSec              float32
	AgeSec               float32
	Radius               float32
	HitUnits             bool
	HitBuilds            bool
	BulletType           int16
	BulletClass          string
	SplashRadius         float32
	BuildingDamage       float32
	ArmorMultiplier      float32
	MaxDamageFraction    float32
	ShieldDamageMul      float32
	PierceDamageFactor   float32
	PierceArmor          bool
	SlowSec              float32
	SlowMul              float32
	PierceRemain         int32
	PierceBuilding       bool
	ChainCount           int32
	ChainRange           float32
	FragmentCount        int32
	FragmentSpread       float32
	FragmentSpeed        float32
	FragmentLife         float32
	FragmentRand         float32
	FragmentAngle        float32
	FragmentVelMin       float32
	FragmentVelMax       float32
	FragmentLifeMin      float32
	FragmentLifeMax      float32
	FragmentBullet       *bulletRuntimeProfile
	StatusID             int16
	StatusName           string
	StatusDuration       float32
	ShootEffect          string
	SmokeEffect          string
	HitEffect            string
	DespawnEffect        string
	TargetAir            bool
	TargetGround         bool
	TargetPriority       string
	HelixScl             float32
	HelixMag             float32
	HelixOffset          float32
	AimX                 float32
	AimY                 float32
	KeepAlive            bool
	DamageTick           float32
	BeamLength           float32
	BeamDamageInterval   float32
	BeamOptimalLifeFract float32
	BeamFadeTime         float32
}

type weaponProfile struct {
	FireMode             string // projectile|beam
	Range                float32
	Damage               float32
	SplashDamage         float32
	Interval             float32
	BulletType           int16
	BulletClass          string
	BulletSpeed          float32
	BulletLifetime       float32
	BulletHitSize        float32
	SplashRadius         float32
	BuildingDamage       float32
	ArmorMultiplier      float32
	MaxDamageFraction    float32
	ShieldDamageMul      float32
	PierceDamageFactor   float32
	PierceArmor          bool
	SlowSec              float32
	SlowMul              float32
	Pierce               int32
	PierceBuilding       bool
	ChainCount           int32
	ChainRange           float32
	StatusID             int16
	StatusName           string
	StatusDuration       float32
	ShootStatusID        int16
	ShootStatusName      string
	ShootStatusDuration  float32
	FragmentCount        int32
	FragmentSpread       float32
	FragmentSpeed        float32
	FragmentLife         float32
	FragmentRandomSpread float32
	FragmentAngle        float32
	FragmentVelocityMin  float32
	FragmentVelocityMax  float32
	FragmentLifeMin      float32
	FragmentLifeMax      float32
	FragmentBullet       *bulletRuntimeProfile
	ShootEffect          string
	SmokeEffect          string
	HitEffect            string
	DespawnEffect        string
	PreferBuildings      bool
	TargetAir            bool
	TargetGround         bool
	TargetPriority       string
	HitBuildings         bool
}

type buildingWeaponProfile struct {
	ClassName            string
	FireMode             string // projectile|beam
	Range                float32
	Damage               float32
	SplashDamage         float32
	Interval             float32
	BulletType           int16
	BulletClass          string
	BulletSpeed          float32
	BulletLifetime       float32
	BulletHitSize        float32
	SplashRadius         float32
	BuildingDamage       float32
	ArmorMultiplier      float32
	MaxDamageFraction    float32
	ShieldDamageMul      float32
	PierceDamageFactor   float32
	PierceArmor          bool
	SlowSec              float32
	SlowMul              float32
	Pierce               int32
	PierceBuilding       bool
	ChainCount           int32
	ChainRange           float32
	StatusID             int16
	StatusName           string
	StatusDuration       float32
	FragmentCount        int32
	FragmentSpread       float32
	FragmentSpeed        float32
	FragmentLife         float32
	FragmentRandomSpread float32
	FragmentAngle        float32
	FragmentVelocityMin  float32
	FragmentVelocityMax  float32
	FragmentLifeMin      float32
	FragmentLifeMax      float32
	FragmentBullet       *bulletRuntimeProfile
	Bullet               *bulletRuntimeProfile
	ShootEffect          string
	SmokeEffect          string
	HitEffect            string
	DespawnEffect        string
	HitBuildings         bool
	TargetBuilds         bool
	TargetAir            bool
	TargetGround         bool
	TargetPriority       string
	MinTargetTeam        TeamID
	Rotate               bool
	RotateSpeed          float32
	BaseRotation         float32
	PredictTarget        bool
	TargetInterval       float32
	TargetSwitchInterval float32
	ShootCone            float32
	RotationLimit        float32

	AmmoCapacity float32
	AmmoRegen    float32
	AmmoPerShot  float32

	PowerCapacity float32
	PowerRegen    float32
	PowerPerShot  float32

	BurstShots   int32
	BurstSpacing float32

	ContinuousHold bool
	AimChangeSpeed float32
	ShootDuration  float32
}

type buildCombatState struct {
	Cooldown       float32
	BurstRemain    int32
	BurstDelay     float32
	Ammo           float32
	Power          float32
	TargetID       int32
	RetargetCD     float32
	TargetBuildPos int32
	TargetBuildOK  bool
	BuildTargetCD  float32
	TurretRotation float32
	HasRotation    bool
	BeamBulletID   int32
	BeamHoldRemain float32
	BeamLastLength float32
}

type targetTrackState struct {
	TargetID   int32
	RetargetCD float32
}

type pendingMountShot struct {
	EntityID    int32
	MountIndex  int
	DelaySec    float32
	XOffset     float32
	YOffset     float32
	AngleOffset float32
	HelixScl    float32
	HelixMag    float32
	HelixOffset float32
}

type unitWeaponMountProfile struct {
	AngleOffset     float32
	CooldownMul     float32
	DamageMul       float32
	RangeMul        float32
	BulletSpeedMul  float32
	BulletType      int16 // -1 means inherit entity bullet type
	SplashRadiusMul float32

	ClassName            string
	FireMode             string
	Range                float32
	Damage               float32
	SplashDamage         float32
	Interval             float32
	BulletClass          string
	BulletSpeed          float32
	BulletLifetime       float32
	BulletHitSize        float32
	SplashRadius         float32
	BuildingDamage       float32
	ArmorMultiplier      float32
	MaxDamageFraction    float32
	ShieldDamageMul      float32
	PierceDamageFactor   float32
	PierceArmor          bool
	SlowSec              float32
	SlowMul              float32
	Pierce               int32
	PierceBuilding       bool
	ChainCount           int32
	ChainRange           float32
	StatusID             int16
	StatusName           string
	StatusDuration       float32
	ShootStatusID        int16
	ShootStatusName      string
	ShootStatusDuration  float32
	FragmentCount        int32
	FragmentSpread       float32
	FragmentSpeed        float32
	FragmentLife         float32
	FragmentRandomSpread float32
	FragmentAngle        float32
	FragmentVelocityMin  float32
	FragmentVelocityMax  float32
	FragmentLifeMin      float32
	FragmentLifeMax      float32
	FragmentBullet       *bulletRuntimeProfile
	ShootEffect          string
	SmokeEffect          string
	HitEffect            string
	DespawnEffect        string
	PreferBuildings      bool
	TargetAir            bool
	TargetGround         bool
	TargetPriority       string
	HitBuildings         bool
	X                    float32
	Y                    float32
	ShootX               float32
	ShootY               float32
	Rotate               bool
	RotateSpeed          float32
	BaseRotation         float32
	Mirror               bool
	Alternate            bool
	FlipSprite           bool
	OtherSide            int32
	Controllable         bool
	AIControllable       bool
	AutoTarget           bool
	PredictTarget        bool
	UseAttackRange       bool
	AlwaysShooting       bool
	NoAttack             bool
	TargetInterval       float32
	TargetSwitchInterval float32
	ShootCone            float32
	MinShootVelocity     float32
	Inaccuracy           float32
	VelocityRnd          float32
	XRand                float32
	YRand                float32
	ExtraVelocity        float32
	RotationLimit        float32
	MinWarmup            float32
	ShootWarmupSpeed     float32
	LinearWarmup         bool
	AimChangeSpeed       float32
	Continuous           bool
	AlwaysContinuous     bool
	PointDefense         bool
	RepairBeam           bool
	TargetUnits          bool
	TargetBuildings      bool
	RepairSpeed          float32
	FractionRepairSpeed  float32
	ShootPattern         string
	ShootShots           int32
	ShootFirstShotDelay  float32
	ShootShotDelay       float32
	ShootSpread          float32
	ShootBarrels         int32
	ShootBarrelOffset    int32
	ShootPatternMirror   bool
	ShootHelixScl        float32
	ShootHelixMag        float32
	ShootHelixOffset     float32
	HitRadius            float32
	Bullet               *bulletRuntimeProfile
}

type unitMountState struct {
	Reload         float32
	Rotation       float32
	TargetRotation float32
	AimX           float32
	AimY           float32
	Side           bool
	Warmup         float32
	BarrelCounter  int32
	TargetID       int32
	TargetBuildPos int32
	RetargetCD     float32
	BeamBulletID   int32
	LastBeamLength float32
}

type vanillaProfilesFile struct {
	Units       []vanillaUnitProfile   `json:"units"`
	UnitsByName []vanillaUnitProfile   `json:"units_by_name"`
	Turrets     []vanillaTurretProfile `json:"turrets"`
	Blocks      []vanillaBlockProfile  `json:"blocks"`
	Statuses    []vanillaStatusProfile `json:"statuses"`
}

type vanillaBlockRequirement struct {
	Item   string  `json:"item"`
	ItemID int16   `json:"item_id"`
	Amount int32   `json:"amount"`
	Cost   float32 `json:"cost"`
}

type vanillaBlockProfile struct {
	Name                string                    `json:"name"`
	Armor               float32                   `json:"armor"`
	BuildCostMultiplier float32                   `json:"build_cost_multiplier"`
	BuildTimeSec        float32                   `json:"build_time_sec"`
	Requirements        []vanillaBlockRequirement `json:"requirements"`
}

type vanillaUnitProfile struct {
	Name                     string                      `json:"name"`
	TypeID                   int16                       `json:"type_id"`
	Constructor              string                      `json:"constructor"`
	EntityComponents         []string                    `json:"entity_components"`
	MovementClass            string                      `json:"movement_class"`
	Naval                    bool                        `json:"naval"`
	Legged                   bool                        `json:"legged"`
	Tank                     bool                        `json:"tank"`
	Hover                    bool                        `json:"hover"`
	Crawl                    bool                        `json:"crawl"`
	PayloadUnit              bool                        `json:"payload_unit"`
	TimedKill                bool                        `json:"timed_kill"`
	BlockUnit                bool                        `json:"block_unit"`
	BuildingTether           bool                        `json:"building_tether"`
	Health                   float32                     `json:"health"`
	Armor                    float32                     `json:"armor"`
	Speed                    float32                     `json:"speed"`
	HitSize                  float32                     `json:"hit_size"`
	RotateSpeed              float32                     `json:"rotate_speed"`
	BuildSpeed               float32                     `json:"build_speed"`
	MineSpeed                float32                     `json:"mine_speed"`
	MineTier                 int16                       `json:"mine_tier"`
	ItemCapacity             int32                       `json:"item_capacity"`
	AmmoCapacity             float32                     `json:"ammo_capacity"`
	AmmoRegen                float32                     `json:"ammo_regen"`
	AmmoPerShot              float32                     `json:"ammo_per_shot"`
	PayloadCapacity          float32                     `json:"payload_capacity"`
	Flying                   bool                        `json:"flying"`
	LowAltitude              bool                        `json:"low_altitude"`
	CanBoost                 bool                        `json:"can_boost"`
	MineWalls                bool                        `json:"mine_walls"`
	MineFloor                bool                        `json:"mine_floor"`
	CoreUnitDock             bool                        `json:"core_unit_dock"`
	AllowedInPayloads        bool                        `json:"allowed_in_payloads"`
	PickupUnits              bool                        `json:"pickup_units"`
	FireMode                 string                      `json:"fire_mode"`
	Range                    float32                     `json:"range"`
	Damage                   float32                     `json:"damage"`
	SplashDamage             float32                     `json:"splash_damage"`
	Interval                 float32                     `json:"interval"`
	BulletType               int16                       `json:"bullet_type"`
	BulletSpeed              float32                     `json:"bullet_speed"`
	BulletLifetime           float32                     `json:"bullet_lifetime"`
	BulletHitSize            float32                     `json:"bullet_hit_size"`
	SplashRadius             float32                     `json:"splash_radius"`
	BuildingDamageMultiplier float32                     `json:"building_damage_multiplier"`
	ArmorMultiplier          float32                     `json:"armor_multiplier"`
	MaxDamageFraction        float32                     `json:"max_damage_fraction"`
	ShieldDamageMultiplier   float32                     `json:"shield_damage_multiplier"`
	PierceDamageFactor       float32                     `json:"pierce_damage_factor"`
	PierceArmor              bool                        `json:"pierce_armor"`
	SlowSec                  float32                     `json:"slow_sec"`
	SlowMul                  float32                     `json:"slow_mul"`
	Pierce                   int32                       `json:"pierce"`
	PierceBuilding           bool                        `json:"pierce_building"`
	ChainCount               int32                       `json:"chain_count"`
	ChainRange               float32                     `json:"chain_range"`
	StatusID                 int16                       `json:"status_id"`
	StatusName               string                      `json:"status_name"`
	StatusDuration           float32                     `json:"status_duration"`
	ShootStatusID            int16                       `json:"shoot_status_id"`
	ShootStatusName          string                      `json:"shoot_status_name"`
	ShootStatusDuration      float32                     `json:"shoot_status_duration"`
	FragmentCount            int32                       `json:"frag_bullets"`
	FragmentSpread           float32                     `json:"frag_spread"`
	FragmentSpeed            float32                     `json:"fragment_speed"`
	FragmentLife             float32                     `json:"fragment_life"`
	FragmentRandomSpread     float32                     `json:"frag_random_spread"`
	FragmentAngle            float32                     `json:"frag_angle"`
	FragmentVelocityMin      float32                     `json:"frag_velocity_min"`
	FragmentVelocityMax      float32                     `json:"frag_velocity_max"`
	FragmentLifeMin          float32                     `json:"frag_life_min"`
	FragmentLifeMax          float32                     `json:"frag_life_max"`
	FragmentBullet           *vanillaBulletProfile       `json:"frag_bullet,omitempty"`
	ShootEffect              string                      `json:"shoot_effect,omitempty"`
	SmokeEffect              string                      `json:"smoke_effect,omitempty"`
	HitEffect                string                      `json:"hit_effect,omitempty"`
	DespawnEffect            string                      `json:"despawn_effect,omitempty"`
	PreferBuildings          bool                        `json:"prefer_buildings"`
	TargetAir                bool                        `json:"target_air"`
	TargetGround             bool                        `json:"target_ground"`
	TargetPriority           string                      `json:"target_priority"`
	HitBuildings             bool                        `json:"hit_buildings"`
	HitRadius                float32                     `json:"hit_radius"`
	Bullet                   *vanillaBulletProfile       `json:"bullet,omitempty"`
	Mounts                   []vanillaWeaponMountProfile `json:"mounts,omitempty"`
	Abilities                []vanillaUnitAbilityProfile `json:"abilities,omitempty"`
}

type vanillaUnitAbilityProfile struct {
	Type                  string  `json:"type"`
	Amount                float32 `json:"amount"`
	Max                   float32 `json:"max"`
	Reload                float32 `json:"reload"`
	Range                 float32 `json:"range"`
	Radius                float32 `json:"radius"`
	Regen                 float32 `json:"regen"`
	Cooldown              float32 `json:"cooldown"`
	Width                 float32 `json:"width"`
	Angle                 float32 `json:"angle"`
	AngleOffset           float32 `json:"angle_offset"`
	X                     float32 `json:"x"`
	Y                     float32 `json:"y"`
	Damage                float32 `json:"damage"`
	StatusID              int16   `json:"status_id"`
	StatusName            string  `json:"status_name,omitempty"`
	StatusDuration        float32 `json:"status_duration"`
	MaxTargets            int32   `json:"max_targets"`
	HealPercent           float32 `json:"heal_percent"`
	SameTypeHealMult      float32 `json:"same_type_heal_mult"`
	ChanceDeflect         float32 `json:"chance_deflect"`
	MissileUnitMultiplier float32 `json:"missile_unit_multiplier"`
	SpawnAmount           int32   `json:"spawn_amount"`
	SpawnRandAmount       int32   `json:"spawn_rand_amount"`
	Spread                float32 `json:"spread"`
	TargetGround          bool    `json:"target_ground"`
	TargetAir             bool    `json:"target_air"`
	HitBuildings          bool    `json:"hit_buildings"`
	HitUnits              bool    `json:"hit_units"`
	Active                bool    `json:"active"`
	WhenShooting          bool    `json:"when_shooting"`
	OnShoot               bool    `json:"on_shoot"`
	UseAmmo               bool    `json:"use_ammo"`
	PushUnits             bool    `json:"push_units"`
	FaceOutwards          bool    `json:"face_outwards"`
	SpawnUnitName         string  `json:"spawn_unit_name,omitempty"`
}

type vanillaTurretProfile struct {
	ClassName                string                `json:"class_name,omitempty"`
	Name                     string                `json:"name"`
	FireMode                 string                `json:"fire_mode"`
	Range                    float32               `json:"range"`
	Damage                   float32               `json:"damage"`
	SplashDamage             float32               `json:"splash_damage"`
	Interval                 float32               `json:"interval"`
	BulletType               int16                 `json:"bullet_type"`
	BulletSpeed              float32               `json:"bullet_speed"`
	BulletLifetime           float32               `json:"bullet_lifetime"`
	BulletHitSize            float32               `json:"bullet_hit_size"`
	SplashRadius             float32               `json:"splash_radius"`
	BuildingDamageMultiplier float32               `json:"building_damage_multiplier"`
	ArmorMultiplier          float32               `json:"armor_multiplier"`
	MaxDamageFraction        float32               `json:"max_damage_fraction"`
	ShieldDamageMultiplier   float32               `json:"shield_damage_multiplier"`
	PierceDamageFactor       float32               `json:"pierce_damage_factor"`
	PierceArmor              bool                  `json:"pierce_armor"`
	SlowSec                  float32               `json:"slow_sec"`
	SlowMul                  float32               `json:"slow_mul"`
	Pierce                   int32                 `json:"pierce"`
	PierceBuilding           bool                  `json:"pierce_building"`
	ChainCount               int32                 `json:"chain_count"`
	ChainRange               float32               `json:"chain_range"`
	StatusID                 int16                 `json:"status_id"`
	StatusName               string                `json:"status_name"`
	StatusDuration           float32               `json:"status_duration"`
	FragmentCount            int32                 `json:"frag_bullets"`
	FragmentSpread           float32               `json:"frag_spread"`
	FragmentSpeed            float32               `json:"fragment_speed"`
	FragmentLife             float32               `json:"fragment_life"`
	FragmentRandomSpread     float32               `json:"frag_random_spread"`
	FragmentAngle            float32               `json:"frag_angle"`
	FragmentVelocityMin      float32               `json:"frag_velocity_min"`
	FragmentVelocityMax      float32               `json:"frag_velocity_max"`
	FragmentLifeMin          float32               `json:"frag_life_min"`
	FragmentLifeMax          float32               `json:"frag_life_max"`
	FragmentBullet           *vanillaBulletProfile `json:"frag_bullet,omitempty"`
	ShootEffect              string                `json:"shoot_effect,omitempty"`
	SmokeEffect              string                `json:"smoke_effect,omitempty"`
	HitEffect                string                `json:"hit_effect,omitempty"`
	DespawnEffect            string                `json:"despawn_effect,omitempty"`
	HitBuildings             bool                  `json:"hit_buildings"`
	TargetBuilds             bool                  `json:"target_builds"`
	TargetAir                bool                  `json:"target_air"`
	TargetGround             bool                  `json:"target_ground"`
	TargetPriority           string                `json:"target_priority"`
	AmmoCapacity             float32               `json:"ammo_capacity"`
	AmmoRegen                float32               `json:"ammo_regen"`
	AmmoPerShot              float32               `json:"ammo_per_shot"`
	PowerCapacity            float32               `json:"power_capacity"`
	PowerRegen               float32               `json:"power_regen"`
	PowerPerShot             float32               `json:"power_per_shot"`
	BurstShots               int32                 `json:"burst_shots"`
	BurstSpacing             float32               `json:"burst_spacing"`
	ContinuousHold           bool                  `json:"continuous_hold"`
	AimChangeSpeed           float32               `json:"aim_change_speed"`
	ShootDuration            float32               `json:"shoot_duration"`
	Rotate                   bool                  `json:"rotate"`
	RotateSpeed              float32               `json:"rotate_speed"`
	BaseRotation             float32               `json:"base_rotation"`
	PredictTarget            bool                  `json:"predict_target"`
	TargetInterval           float32               `json:"target_interval"`
	TargetSwitchInterval     float32               `json:"target_switch_interval"`
	ShootCone                float32               `json:"shoot_cone"`
	RotationLimit            float32               `json:"rotation_limit"`
	Bullet                   *vanillaBulletProfile `json:"bullet,omitempty"`
}

type vanillaBulletProfile struct {
	ClassName                string                `json:"class_name,omitempty"`
	Damage                   float32               `json:"damage"`
	SplashDamage             float32               `json:"splash_damage"`
	BulletType               int16                 `json:"bullet_type"`
	Speed                    float32               `json:"speed"`
	Lifetime                 float32               `json:"lifetime"`
	HitSize                  float32               `json:"hit_size"`
	SplashRadius             float32               `json:"splash_radius"`
	BuildingDamageMultiplier float32               `json:"building_damage_multiplier"`
	ArmorMultiplier          float32               `json:"armor_multiplier"`
	MaxDamageFraction        float32               `json:"max_damage_fraction"`
	ShieldDamageMultiplier   float32               `json:"shield_damage_multiplier"`
	PierceDamageFactor       float32               `json:"pierce_damage_factor"`
	PierceArmor              bool                  `json:"pierce_armor"`
	Pierce                   int32                 `json:"pierce"`
	PierceBuilding           bool                  `json:"pierce_building"`
	StatusID                 int16                 `json:"status_id"`
	StatusName               string                `json:"status_name"`
	StatusDuration           float32               `json:"status_duration"`
	HitBuildings             bool                  `json:"hit_buildings"`
	TargetAir                bool                  `json:"target_air"`
	TargetGround             bool                  `json:"target_ground"`
	ShootEffect              string                `json:"shoot_effect,omitempty"`
	SmokeEffect              string                `json:"smoke_effect,omitempty"`
	HitEffect                string                `json:"hit_effect,omitempty"`
	DespawnEffect            string                `json:"despawn_effect,omitempty"`
	Length                   float32               `json:"length"`
	DamageInterval           float32               `json:"damage_interval"`
	OptimalLifeFract         float32               `json:"optimal_life_fract"`
	FadeTime                 float32               `json:"fade_time"`
	FragBullets              int32                 `json:"frag_bullets"`
	FragSpread               float32               `json:"frag_spread"`
	FragRandomSpread         float32               `json:"frag_random_spread"`
	FragAngle                float32               `json:"frag_angle"`
	FragVelocityMin          float32               `json:"frag_velocity_min"`
	FragVelocityMax          float32               `json:"frag_velocity_max"`
	FragLifeMin              float32               `json:"frag_life_min"`
	FragLifeMax              float32               `json:"frag_life_max"`
	FragBullet               *vanillaBulletProfile `json:"frag_bullet,omitempty"`
}

type vanillaWeaponMountProfile struct {
	ClassName                string                `json:"class_name,omitempty"`
	FireMode                 string                `json:"fire_mode"`
	Range                    float32               `json:"range"`
	Damage                   float32               `json:"damage"`
	SplashDamage             float32               `json:"splash_damage"`
	Interval                 float32               `json:"interval"`
	BulletType               int16                 `json:"bullet_type"`
	BulletSpeed              float32               `json:"bullet_speed"`
	BulletLifetime           float32               `json:"bullet_lifetime"`
	BulletHitSize            float32               `json:"bullet_hit_size"`
	SplashRadius             float32               `json:"splash_radius"`
	BuildingDamageMultiplier float32               `json:"building_damage_multiplier"`
	ArmorMultiplier          float32               `json:"armor_multiplier"`
	MaxDamageFraction        float32               `json:"max_damage_fraction"`
	ShieldDamageMultiplier   float32               `json:"shield_damage_multiplier"`
	PierceDamageFactor       float32               `json:"pierce_damage_factor"`
	PierceArmor              bool                  `json:"pierce_armor"`
	Pierce                   int32                 `json:"pierce"`
	PierceBuilding           bool                  `json:"pierce_building"`
	StatusID                 int16                 `json:"status_id"`
	StatusName               string                `json:"status_name"`
	StatusDuration           float32               `json:"status_duration"`
	FragBullets              int32                 `json:"frag_bullets"`
	FragSpread               float32               `json:"frag_spread"`
	FragRandomSpread         float32               `json:"frag_random_spread"`
	FragAngle                float32               `json:"frag_angle"`
	FragVelocityMin          float32               `json:"frag_velocity_min"`
	FragVelocityMax          float32               `json:"frag_velocity_max"`
	FragLifeMin              float32               `json:"frag_life_min"`
	FragLifeMax              float32               `json:"frag_life_max"`
	FragBullet               *vanillaBulletProfile `json:"frag_bullet,omitempty"`
	TargetAir                bool                  `json:"target_air"`
	TargetGround             bool                  `json:"target_ground"`
	HitBuildings             bool                  `json:"hit_buildings"`
	PreferBuildings          bool                  `json:"prefer_buildings"`
	HitRadius                float32               `json:"hit_radius"`
	ShootStatusID            int16                 `json:"shoot_status_id"`
	ShootStatusName          string                `json:"shoot_status_name"`
	ShootStatusDuration      float32               `json:"shoot_status_duration"`
	ShootEffect              string                `json:"shoot_effect,omitempty"`
	SmokeEffect              string                `json:"smoke_effect,omitempty"`
	HitEffect                string                `json:"hit_effect,omitempty"`
	DespawnEffect            string                `json:"despawn_effect,omitempty"`
	Bullet                   *vanillaBulletProfile `json:"bullet,omitempty"`
	X                        float32               `json:"x"`
	Y                        float32               `json:"y"`
	ShootX                   float32               `json:"shoot_x"`
	ShootY                   float32               `json:"shoot_y"`
	Rotate                   bool                  `json:"rotate"`
	RotateSpeed              float32               `json:"rotate_speed"`
	BaseRotation             float32               `json:"base_rotation"`
	Mirror                   bool                  `json:"mirror"`
	Alternate                bool                  `json:"alternate"`
	FlipSprite               bool                  `json:"flip_sprite"`
	OtherSide                int32                 `json:"other_side"`
	Controllable             bool                  `json:"controllable"`
	AIControllable           bool                  `json:"ai_controllable"`
	AutoTarget               bool                  `json:"auto_target"`
	PredictTarget            bool                  `json:"predict_target"`
	UseAttackRange           bool                  `json:"use_attack_range"`
	AlwaysShooting           bool                  `json:"always_shooting"`
	NoAttack                 bool                  `json:"no_attack"`
	TargetInterval           float32               `json:"target_interval"`
	TargetSwitchInterval     float32               `json:"target_switch_interval"`
	ShootCone                float32               `json:"shoot_cone"`
	MinShootVelocity         float32               `json:"min_shoot_velocity"`
	Inaccuracy               float32               `json:"inaccuracy"`
	VelocityRnd              float32               `json:"velocity_rnd"`
	XRand                    float32               `json:"x_rand"`
	YRand                    float32               `json:"y_rand"`
	ExtraVelocity            float32               `json:"extra_velocity"`
	RotationLimit            float32               `json:"rotation_limit"`
	MinWarmup                float32               `json:"min_warmup"`
	ShootWarmupSpeed         float32               `json:"shoot_warmup_speed"`
	LinearWarmup             bool                  `json:"linear_warmup"`
	AimChangeSpeed           float32               `json:"aim_change_speed"`
	Continuous               bool                  `json:"continuous"`
	AlwaysContinuous         bool                  `json:"always_continuous"`
	PointDefense             bool                  `json:"point_defense"`
	RepairBeam               bool                  `json:"repair_beam"`
	TargetUnits              bool                  `json:"target_units"`
	TargetBuildings          bool                  `json:"target_buildings"`
	RepairSpeed              float32               `json:"repair_speed"`
	FractionRepairSpeed      float32               `json:"fraction_repair_speed"`
	ShootPattern             string                `json:"shoot_pattern,omitempty"`
	ShootShots               int32                 `json:"shoot_shots"`
	ShootFirstShotDelay      float32               `json:"shoot_first_shot_delay"`
	ShootShotDelay           float32               `json:"shoot_shot_delay"`
	ShootSpread              float32               `json:"shoot_spread"`
	ShootBarrels             int32                 `json:"shoot_barrels"`
	ShootBarrelOffset        int32                 `json:"shoot_barrel_offset"`
	ShootPatternMirror       bool                  `json:"shoot_pattern_mirror"`
	ShootHelixScl            float32               `json:"shoot_helix_scl"`
	ShootHelixMag            float32               `json:"shoot_helix_mag"`
	ShootHelixOffset         float32               `json:"shoot_helix_offset"`
}

type vanillaStatusProfile struct {
	ID                   int16    `json:"id"`
	Name                 string   `json:"name"`
	DamageMultiplier     float32  `json:"damage_multiplier"`
	HealthMultiplier     float32  `json:"health_multiplier"`
	SpeedMultiplier      float32  `json:"speed_multiplier"`
	ReloadMultiplier     float32  `json:"reload_multiplier"`
	BuildSpeedMultiplier float32  `json:"build_speed_multiplier"`
	DragMultiplier       float32  `json:"drag_multiplier"`
	TransitionDamage     float32  `json:"transition_damage"`
	Damage               float32  `json:"damage"`
	IntervalDamageTime   float32  `json:"interval_damage_time"`
	IntervalDamage       float32  `json:"interval_damage"`
	IntervalDamagePierce bool     `json:"interval_damage_pierce"`
	Disarm               bool     `json:"disarm"`
	Permanent            bool     `json:"permanent"`
	Reactive             bool     `json:"reactive"`
	Dynamic              bool     `json:"dynamic"`
	Opposites            []string `json:"opposites,omitempty"`
	Affinities           []string `json:"affinities,omitempty"`
}

var defaultWeaponProfile = weaponProfile{
	FireMode:            "projectile",
	Range:               56,
	Damage:              8,
	Interval:            0.7,
	BulletType:          0,
	BulletSpeed:         34,
	BulletHitSize:       10,
	SplashRadius:        0,
	BuildingDamage:      1,
	SlowSec:             0,
	SlowMul:             1,
	Pierce:              0,
	ChainCount:          0,
	ChainRange:          0,
	FragmentCount:       0,
	FragmentSpread:      0,
	FragmentSpeed:       0,
	FragmentLife:        0,
	FragmentVelocityMin: 0.2,
	FragmentVelocityMax: 1,
	FragmentLifeMin:     1,
	FragmentLifeMax:     1,
	PreferBuildings:     false,
	TargetAir:           true,
	TargetGround:        true,
	TargetPriority:      "nearest",
	HitBuildings:        true,
}

// Approximate presets by typeId to make combat behavior more varied.
var weaponProfilesByType = map[int16]weaponProfile{
	0:  {FireMode: "projectile", Range: 64, Damage: 10, Interval: 0.60, BulletType: 0, BulletSpeed: 36, TargetAir: true, TargetGround: true, HitBuildings: true},
	1:  {FireMode: "projectile", Range: 72, Damage: 12, Interval: 0.55, BulletType: 1, BulletSpeed: 40, Pierce: 1, TargetAir: true, TargetGround: true, HitBuildings: true},
	2:  {FireMode: "projectile", Range: 88, Damage: 20, Interval: 1.10, BulletType: 2, BulletSpeed: 46, SplashRadius: 14, TargetAir: false, TargetGround: true, HitBuildings: true},
	3:  {FireMode: "projectile", Range: 68, Damage: 9, Interval: 0.40, BulletType: 3, BulletSpeed: 44, TargetAir: true, TargetGround: false, HitBuildings: false},
	4:  {FireMode: "projectile", Range: 76, Damage: 11, Interval: 0.75, BulletType: 4, BulletSpeed: 38, SlowSec: 1.8, SlowMul: 0.65, ChainCount: 2, ChainRange: 28, TargetAir: false, TargetGround: true, HitBuildings: true},
	5:  {FireMode: "beam", Range: 96, Damage: 16, Interval: 0.90, BulletType: 5, BulletSpeed: 52, TargetAir: true, TargetGround: true, HitBuildings: false},
	6:  {FireMode: "projectile", Range: 80, Damage: 14, Interval: 0.80, BulletType: 6, BulletSpeed: 42, SplashRadius: 10, Pierce: 1, TargetAir: false, TargetGround: true, HitBuildings: true},
	7:  {FireMode: "projectile", Range: 120, Damage: 24, Interval: 1.30, BulletType: 7, BulletSpeed: 58, FragmentCount: 3, FragmentSpread: 24, FragmentSpeed: 34, FragmentLife: 0.6, TargetAir: true, TargetGround: true, HitBuildings: true},
	8:  {FireMode: "projectile", Range: 54, Damage: 7, Interval: 0.32, BulletType: 8, BulletSpeed: 36, TargetAir: false, TargetGround: true, HitBuildings: false},
	9:  {FireMode: "projectile", Range: 92, Damage: 15, Interval: 0.95, BulletType: 9, BulletSpeed: 48, SlowSec: 2.2, SlowMul: 0.55, ChainCount: 3, ChainRange: 34, TargetAir: true, TargetGround: true, HitBuildings: true},
	10: {FireMode: "projectile", Range: 66, Damage: 10, Interval: 0.50, BulletType: 10, BulletSpeed: 40, TargetAir: true, TargetGround: true, HitBuildings: true},
	11: {FireMode: "beam", Range: 132, Damage: 28, Interval: 1.35, BulletType: 11, TargetAir: true, TargetGround: true, TargetPriority: "threat", HitBuildings: true},
	12: {FireMode: "projectile", Range: 72, Damage: 13, Interval: 0.70, BulletType: 12, BulletSpeed: 43, PreferBuildings: true, TargetAir: false, TargetGround: true, TargetPriority: "lowest_health", HitBuildings: true},
	13: {FireMode: "projectile", Range: 58, Damage: 8, Interval: 0.30, BulletType: 13, BulletSpeed: 37, Pierce: 2, TargetAir: true, TargetGround: false, HitBuildings: false},
	14: {FireMode: "projectile", Range: 100, Damage: 19, Interval: 1.00, BulletType: 14, BulletSpeed: 50, SplashRadius: 16, PreferBuildings: true, TargetAir: false, TargetGround: true, HitBuildings: true},
	15: {FireMode: "projectile", Range: 84, Damage: 16, Interval: 0.82, BulletType: 15, BulletSpeed: 46, FragmentCount: 4, FragmentSpread: 32, FragmentSpeed: 30, FragmentLife: 0.75, TargetAir: true, TargetGround: true, TargetPriority: "threat", HitBuildings: true},
}

// Vanilla turret block-name profiles (approximate baseline).
var buildingWeaponProfilesByName = map[string]buildingWeaponProfile{
	"duo":        {FireMode: "projectile", Range: 136, Damage: 9, Interval: 0.27, BulletType: 94, BulletSpeed: 54, TargetAir: true, TargetGround: true, HitBuildings: true, AmmoCapacity: 80, AmmoRegen: 3.0, AmmoPerShot: 1, BurstShots: 2, BurstSpacing: 0.06},
	"scatter":    {FireMode: "projectile", Range: 152, Damage: 7, Interval: 0.23, BulletType: 99, BulletSpeed: 57, TargetAir: true, TargetGround: false, HitBuildings: false, AmmoCapacity: 30, AmmoRegen: 2.8, AmmoPerShot: 1, BurstShots: 3, BurstSpacing: 0.04},
	"scorch":     {FireMode: "projectile", Range: 62, Damage: 16, Interval: 0.13, BulletType: 101, BulletSpeed: 42, TargetAir: false, TargetGround: true, HitBuildings: false, AmmoCapacity: 30, AmmoRegen: 2.2, AmmoPerShot: 1},
	"hail":       {FireMode: "projectile", Range: 236, Damage: 24, Interval: 1.20, BulletType: 103, BulletSpeed: 52, SplashRadius: 18, TargetAir: false, TargetGround: true, HitBuildings: true, AmmoCapacity: 30, AmmoRegen: 1.1, AmmoPerShot: 1},
	"wave":       {FireMode: "projectile", Range: 118, Damage: 4, Interval: 0.09, BulletType: 106, BulletSpeed: 38, SlowSec: 1.8, SlowMul: 0.6, TargetAir: false, TargetGround: true, HitBuildings: false},
	"lancer":     {FireMode: "beam", Range: 172, Damage: 96, Interval: 1.35, BulletType: 110, TargetAir: true, TargetGround: true, TargetPriority: "threat", HitBuildings: true, PowerCapacity: 280, PowerRegen: 22, PowerPerShot: 80},
	"arc":        {FireMode: "beam", Range: 88, Damage: 24, Interval: 0.42, BulletType: 111, ChainCount: 2, ChainRange: 32, HitBuildings: true, PowerCapacity: 140, PowerRegen: 16, PowerPerShot: 30},
	"parallax":   {FireMode: "projectile", Range: 292, Damage: 20, Interval: 0.55, BulletType: 112, BulletSpeed: 64, SlowSec: 0.8, SlowMul: 0.75, TargetAir: true, TargetGround: false, HitBuildings: false},
	"swarmer":    {FireMode: "projectile", Range: 216, Damage: 22, Interval: 0.35, BulletType: 113, BulletSpeed: 62, SplashRadius: 12, HitBuildings: true, AmmoCapacity: 30, AmmoRegen: 1.7, AmmoPerShot: 1, BurstShots: 2, BurstSpacing: 0.05},
	"salvo":      {FireMode: "projectile", Range: 188, Damage: 23, Interval: 0.32, BulletType: 116, BulletSpeed: 60, Pierce: 1, HitBuildings: true, AmmoCapacity: 30, AmmoRegen: 2.0, AmmoPerShot: 1, BurstShots: 4, BurstSpacing: 0.045},
	"segment":    {FireMode: "beam", Range: 88, Damage: 26, Interval: 0.16, BulletType: 111, ChainCount: 1, ChainRange: 20, TargetAir: true, TargetGround: false, HitBuildings: false},
	"tsunami":    {FireMode: "projectile", Range: 174, Damage: 10, Interval: 0.08, BulletType: 106, BulletSpeed: 44, SlowSec: 2.8, SlowMul: 0.45, TargetAir: false, TargetGround: true, HitBuildings: false},
	"fuse":       {FireMode: "beam", Range: 120, Damage: 180, Interval: 0.95, BulletType: 125, HitBuildings: true, AmmoCapacity: 30, AmmoRegen: 1.2, AmmoPerShot: 1},
	"ripple":     {FireMode: "projectile", Range: 286, Damage: 62, Interval: 1.35, BulletType: 127, BulletSpeed: 72, SplashRadius: 24, HitBuildings: true, AmmoCapacity: 30, AmmoRegen: 0.9, AmmoPerShot: 2},
	"cyclone":    {FireMode: "projectile", Range: 214, Damage: 18, Interval: 0.10, BulletType: 133, BulletSpeed: 65, HitBuildings: true, AmmoCapacity: 30, AmmoRegen: 4.8, AmmoPerShot: 1},
	"foreshadow": {FireMode: "projectile", Range: 472, Damage: 640, Interval: 4.8, BulletType: 139, BulletSpeed: 94, Pierce: 3, TargetPriority: "highest_health", HitBuildings: true, AmmoCapacity: 40, AmmoRegen: 0.8, AmmoPerShot: 5, PowerCapacity: 1800, PowerRegen: 90, PowerPerShot: 900},
	"spectre":    {FireMode: "projectile", Range: 300, Damage: 84, Interval: 0.18, BulletType: 140, BulletSpeed: 82, TargetPriority: "threat", HitBuildings: true, AmmoCapacity: 30, AmmoRegen: 3.4, AmmoPerShot: 1},
	"meltdown":   {FireMode: "beam", Range: 236, Damage: 94, Interval: 0.12, BulletType: 143, SlowSec: 0.7, SlowMul: 0.8, HitBuildings: true, PowerCapacity: 1200, PowerRegen: 120, PowerPerShot: 60},
	"breach":     {FireMode: "projectile", Range: 120, Damage: 25, Interval: 0.22, BulletType: 144, BulletSpeed: 56, HitBuildings: true, AmmoCapacity: 30, AmmoPerShot: 2},
	"diffuse":    {FireMode: "projectile", Range: 152, Damage: 16, Interval: 0.14, BulletType: 148, BulletSpeed: 58, HitBuildings: true, AmmoCapacity: 30, AmmoPerShot: 3},
	"sublimate":  {FireMode: "beam", Range: 156, Damage: 52, Interval: 0.22, BulletType: 151, ChainCount: 2, ChainRange: 28, HitBuildings: true, PowerCapacity: 360, PowerRegen: 28, PowerPerShot: 36},
	"titan":      {FireMode: "projectile", Range: 210, Damage: 38, Interval: 0.36, BulletType: 153, BulletSpeed: 66, HitBuildings: true, AmmoCapacity: 12, AmmoPerShot: 4},
	"disperse":   {FireMode: "projectile", Range: 230, Damage: 36, Interval: 0.28, BulletType: 159, BulletSpeed: 72, SplashRadius: 18, HitBuildings: true, AmmoCapacity: 30, AmmoPerShot: 1},
	"afflict":    {FireMode: "beam", Range: 246, Damage: 128, Interval: 0.24, BulletType: 164, HitBuildings: true, PowerCapacity: 760, PowerRegen: 62, PowerPerShot: 84},
	"lustre":     {FireMode: "beam", Range: 332, Damage: 180, Interval: 0.26, BulletType: 166, ChainCount: 1, ChainRange: 36, HitBuildings: true, PowerCapacity: 980, PowerRegen: 70, PowerPerShot: 100},
	"scathe":     {FireMode: "projectile", Range: 438, Damage: 260, Interval: 1.05, BulletType: 167, BulletSpeed: 84, SplashRadius: 26, HitBuildings: true, TargetBuilds: true, AmmoCapacity: 45, AmmoRegen: 0.55, AmmoPerShot: 15},
	"smite":      {FireMode: "projectile", Range: 352, Damage: 220, Interval: 0.65, BulletType: 177, BulletSpeed: 86, SplashRadius: 20, HitBuildings: true, AmmoCapacity: 30, AmmoRegen: 0.75, AmmoPerShot: 2},
	"malign":     {FireMode: "beam", Range: 402, Damage: 260, Interval: 0.34, BulletType: 180, ChainCount: 2, ChainRange: 44, HitBuildings: true, PowerCapacity: 1400, PowerRegen: 105, PowerPerShot: 140},
}

var turretItemAmmoBulletTypesByName = map[string]map[ItemID]int16{
	"duo": {
		copperItemID:   94,
		graphiteItemID: 95,
		siliconItemID:  96,
	},
	"scatter": {
		scrapItemID:     97,
		leadItemID:      98,
		metaglassItemID: 99,
	},
	"scorch": {
		coalItemID:     101,
		pyratiteItemID: 102,
	},
	"hail": {
		graphiteItemID: 103,
		siliconItemID:  104,
		pyratiteItemID: 105,
	},
	"swarmer": {
		blastCompoundItemID: 113,
		pyratiteItemID:      114,
		surgeAlloyItemID:    115,
	},
	"salvo": {
		copperItemID:   116,
		graphiteItemID: 117,
		pyratiteItemID: 118,
		siliconItemID:  119,
		thoriumItemID:  120,
	},
	"fuse": {
		titaniumItemID: 125,
		thoriumItemID:  126,
	},
	"ripple": {
		graphiteItemID:      127,
		siliconItemID:       128,
		pyratiteItemID:      129,
		blastCompoundItemID: 130,
		plastaniumItemID:    131,
	},
	"cyclone": {
		metaglassItemID:     133,
		blastCompoundItemID: 135,
		plastaniumItemID:    136,
		surgeAlloyItemID:    138,
	},
	"spectre": {
		graphiteItemID: 140,
		thoriumItemID:  141,
		pyratiteItemID: 142,
	},
	"breach": {
		berylliumItemID: 144,
		tungstenItemID:  145,
		carbideItemID:   146,
	},
	"diffuse": {
		graphiteItemID: 148,
		oxideItemID:    149,
		siliconItemID:  150,
	},
	"titan": {
		thoriumItemID: 153,
		carbideItemID: 154,
		oxideItemID:   156,
	},
	"disperse": {
		tungstenItemID:   159,
		thoriumItemID:    160,
		siliconItemID:    161,
		surgeAlloyItemID: 162,
	},
	"scathe": {
		carbideItemID:     167,
		phaseFabricItemID: 170,
		surgeAlloyItemID:  173,
	},
	"smite": {
		surgeAlloyItemID: 177,
	},
}

var turretLiquidAmmoBulletTypesByName = map[string]map[LiquidID]int16{
	"wave": {
		waterLiquidID:     106,
		slagLiquidID:      107,
		cryofluidLiquidID: 108,
		oilLiquidID:       109,
	},
	"tsunami": {
		waterLiquidID:     106,
		slagLiquidID:      107,
		cryofluidLiquidID: 108,
		oilLiquidID:       109,
	},
	"sublimate": {
		ozoneLiquidID:    151,
		cyanogenLiquidID: 152,
	},
}

// Approximate multi-mount presets by unit typeId.
var unitMountProfilesByType = map[int16][]unitWeaponMountProfile{
	3: {
		{AngleOffset: -8, CooldownMul: 1.00, DamageMul: 0.55, RangeMul: 1.0, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
		{AngleOffset: 8, CooldownMul: 1.00, DamageMul: 0.55, RangeMul: 1.0, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
	},
	6: {
		{AngleOffset: -5, CooldownMul: 0.95, DamageMul: 0.7, RangeMul: 1.0, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
		{AngleOffset: 5, CooldownMul: 0.95, DamageMul: 0.7, RangeMul: 1.0, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
	},
	7: {
		{AngleOffset: -12, CooldownMul: 1.10, DamageMul: 0.62, RangeMul: 1.0, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
		{AngleOffset: 0, CooldownMul: 1.05, DamageMul: 0.72, RangeMul: 1.0, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
		{AngleOffset: 12, CooldownMul: 1.10, DamageMul: 0.62, RangeMul: 1.0, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
	},
	11: {
		{AngleOffset: 0, CooldownMul: 1.00, DamageMul: 0.7, RangeMul: 1.0, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
		{AngleOffset: -18, CooldownMul: 1.25, DamageMul: 0.35, RangeMul: 0.92, BulletSpeedMul: 1.0, BulletType: 10, SplashRadiusMul: 0.7},
		{AngleOffset: 18, CooldownMul: 1.25, DamageMul: 0.35, RangeMul: 0.92, BulletSpeedMul: 1.0, BulletType: 10, SplashRadiusMul: 0.7},
	},
	15: {
		{AngleOffset: -10, CooldownMul: 1.00, DamageMul: 0.52, RangeMul: 1.05, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
		{AngleOffset: 10, CooldownMul: 1.00, DamageMul: 0.52, RangeMul: 1.05, BulletSpeedMul: 1.0, BulletType: -1, SplashRadiusMul: 1},
	},
}

var entityHitRadiusByType = map[int16]float32{
	0:  4.0,
	1:  4.5,
	2:  5.0,
	3:  5.2,
	4:  4.6,
	5:  5.4,
	6:  6.0,
	7:  6.6,
	8:  3.8,
	9:  5.8,
	10: 5.0,
	11: 7.0,
	12: 5.6,
	13: 4.2,
	14: 6.4,
	15: 7.4,
}

func (w *World) LoadVanillaProfiles(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload vanillaProfilesFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if len(payload.Units) > 0 {
		base := cloneUnitWeaponProfiles(weaponProfilesByType)
		byName := cloneUnitWeaponProfilesByName(w.unitProfilesByName)
		metaByName := make(map[string]unitRuntimeProfile, len(w.unitRuntimeProfilesByName))
		for k, v := range w.unitRuntimeProfilesByName {
			metaByName[k] = cloneUnitRuntimeProfile(v)
		}
		mountsByName := cloneUnitMountProfilesByName(w.unitMountProfilesByName)
		for _, u := range payload.Units {
			name := strings.ToLower(strings.TrimSpace(u.Name))
			if name != "" {
				pn := defaultWeaponProfile
				if cur, ok := byName[name]; ok {
					pn = cur
				}
				mergeUnitProfile(&pn, u)
				byName[name] = pn
				if len(u.Mounts) > 0 {
					parsed := make([]unitWeaponMountProfile, 0, len(u.Mounts))
					for _, m := range u.Mounts {
						parsed = append(parsed, convertVanillaMountProfile(m))
					}
					mountsByName[name] = parsed
				}
				metaByName[name] = convertVanillaUnitRuntimeProfile(u)
			}
			if u.TypeID >= 0 {
				p := defaultWeaponProfile
				if cur, ok := base[u.TypeID]; ok {
					p = cur
				}
				mergeUnitProfile(&p, u)
				base[u.TypeID] = p
				if u.HitRadius > 0 {
					entityHitRadiusByType[u.TypeID] = u.HitRadius
				}
			}
		}
		w.unitProfilesByType = base
		w.unitProfilesByName = byName
		w.unitRuntimeProfilesByName = metaByName
		w.unitMountProfilesByName = mountsByName
	}
	if len(payload.UnitsByName) > 0 {
		base := cloneUnitWeaponProfilesByName(w.unitProfilesByName)
		metaByName := make(map[string]unitRuntimeProfile, len(w.unitRuntimeProfilesByName))
		for k, v := range w.unitRuntimeProfilesByName {
			metaByName[k] = cloneUnitRuntimeProfile(v)
		}
		mountsByName := cloneUnitMountProfilesByName(w.unitMountProfilesByName)
		for _, u := range payload.UnitsByName {
			name := strings.ToLower(strings.TrimSpace(u.Name))
			if name == "" {
				continue
			}
			p := defaultWeaponProfile
			if cur, ok := base[name]; ok {
				p = cur
			}
			mergeUnitProfile(&p, u)
			base[name] = p
			if len(u.Mounts) > 0 {
				parsed := make([]unitWeaponMountProfile, 0, len(u.Mounts))
				for _, m := range u.Mounts {
					parsed = append(parsed, convertVanillaMountProfile(m))
				}
				mountsByName[name] = parsed
			}
			metaByName[name] = convertVanillaUnitRuntimeProfile(u)
		}
		w.unitProfilesByName = base
		w.unitRuntimeProfilesByName = metaByName
		w.unitMountProfilesByName = mountsByName
	}
	if len(payload.Turrets) > 0 {
		base := cloneBuildingWeaponProfiles(buildingWeaponProfilesByName)
		for _, t := range payload.Turrets {
			name := strings.ToLower(strings.TrimSpace(t.Name))
			if name == "" {
				continue
			}
			p := buildingWeaponProfile{}
			if cur, ok := base[name]; ok {
				p = cur
			}
			mergeBuildingProfile(&p, t)
			base[name] = p
		}
		w.buildingProfilesByName = base
	}
	if len(payload.Blocks) > 0 {
		costs := make(map[string][]ItemStack, len(payload.Blocks))
		times := make(map[string]float32, len(payload.Blocks))
		armor := make(map[string]float32, len(payload.Blocks))
		for _, b := range payload.Blocks {
			name := strings.ToLower(strings.TrimSpace(b.Name))
			if name == "" {
				continue
			}
			if b.Armor > 0 {
				armor[name] = b.Armor
			}
			if b.BuildTimeSec > 0 {
				times[name] = b.BuildTimeSec
			}
			if len(b.Requirements) == 0 {
				continue
			}
			items := make([]ItemStack, 0, len(b.Requirements))
			for _, r := range b.Requirements {
				if r.Amount <= 0 || r.ItemID < 0 {
					continue
				}
				items = append(items, ItemStack{Item: ItemID(r.ItemID), Amount: r.Amount})
			}
			if len(items) > 0 {
				costs[name] = items
			}
		}
		if len(costs) > 0 {
			w.blockCostsByName = costs
		}
		if len(times) > 0 {
			w.blockBuildTimesByName = times
		}
		w.blockArmorByName = armor
	}
	if len(payload.Statuses) > 0 {
		byID := make(map[int16]statusEffectProfile, len(payload.Statuses))
		byName := make(map[string]statusEffectProfile, len(payload.Statuses))
		for _, s := range payload.Statuses {
			prof := statusEffectProfile{
				ID:                   s.ID,
				Name:                 strings.ToLower(strings.TrimSpace(s.Name)),
				DamageMultiplier:     s.DamageMultiplier,
				HealthMultiplier:     s.HealthMultiplier,
				SpeedMultiplier:      s.SpeedMultiplier,
				ReloadMultiplier:     s.ReloadMultiplier,
				BuildSpeedMultiplier: s.BuildSpeedMultiplier,
				DragMultiplier:       s.DragMultiplier,
				TransitionDamage:     s.TransitionDamage,
				Damage:               s.Damage,
				IntervalDamageTime:   s.IntervalDamageTime,
				IntervalDamage:       s.IntervalDamage,
				IntervalDamagePierce: s.IntervalDamagePierce,
				Disarm:               s.Disarm,
				Permanent:            s.Permanent,
				Reactive:             s.Reactive,
				Dynamic:              s.Dynamic,
				Opposites:            append([]string(nil), s.Opposites...),
				Affinities:           append([]string(nil), s.Affinities...),
			}
			byID[prof.ID] = prof
			if prof.Name != "" {
				byName[prof.Name] = prof
			}
		}
		w.statusProfilesByID = byID
		w.statusProfilesByName = byName
	}
	return nil
}

func cloneUnitWeaponProfiles(src map[int16]weaponProfile) map[int16]weaponProfile {
	out := make(map[int16]weaponProfile, len(src))
	for k, v := range src {
		v.FragmentBullet = cloneBulletRuntimeProfile(v.FragmentBullet)
		out[k] = v
	}
	return out
}

func cloneBuildingWeaponProfiles(src map[string]buildingWeaponProfile) map[string]buildingWeaponProfile {
	out := make(map[string]buildingWeaponProfile, len(src))
	for k, v := range src {
		v.FragmentBullet = cloneBulletRuntimeProfile(v.FragmentBullet)
		v.Bullet = cloneBulletRuntimeProfile(v.Bullet)
		out[k] = v
	}
	return out
}

func cloneUnitWeaponProfilesByName(src map[string]weaponProfile) map[string]weaponProfile {
	out := make(map[string]weaponProfile, len(src))
	for k, v := range src {
		v.FragmentBullet = cloneBulletRuntimeProfile(v.FragmentBullet)
		out[k] = v
	}
	return out
}

func mergeUnitProfile(p *weaponProfile, u vanillaUnitProfile) {
	if p == nil {
		return
	}
	if strings.TrimSpace(u.FireMode) != "" {
		p.FireMode = strings.TrimSpace(u.FireMode)
	}
	if u.Range > 0 {
		p.Range = u.Range
	}
	if u.Damage > 0 {
		p.Damage = u.Damage
	}
	if u.SplashDamage > 0 {
		p.SplashDamage = u.SplashDamage
	}
	if u.Interval > 0 {
		p.Interval = u.Interval
	}
	if u.BulletType > 0 {
		p.BulletType = u.BulletType
	}
	p.BulletClass = ""
	if u.Bullet != nil {
		p.BulletClass = strings.TrimSpace(u.Bullet.ClassName)
	}
	if u.BulletSpeed > 0 {
		p.BulletSpeed = u.BulletSpeed
	}
	if u.BulletLifetime > 0 {
		p.BulletLifetime = u.BulletLifetime
	}
	if u.BulletHitSize > 0 {
		p.BulletHitSize = u.BulletHitSize
	}
	p.SplashRadius = u.SplashRadius
	p.BuildingDamage = u.BuildingDamageMultiplier
	p.ArmorMultiplier = u.ArmorMultiplier
	p.MaxDamageFraction = u.MaxDamageFraction
	p.ShieldDamageMul = u.ShieldDamageMultiplier
	p.PierceDamageFactor = u.PierceDamageFactor
	p.PierceArmor = u.PierceArmor
	p.SlowSec = u.SlowSec
	if u.SlowMul > 0 {
		p.SlowMul = u.SlowMul
	}
	p.Pierce = u.Pierce
	p.PierceBuilding = u.PierceBuilding
	p.ChainCount = u.ChainCount
	p.ChainRange = u.ChainRange
	p.FragmentCount = u.FragmentCount
	p.FragmentSpread = u.FragmentSpread
	p.FragmentSpeed = u.FragmentSpeed
	p.FragmentLife = u.FragmentLife
	p.FragmentRandomSpread = u.FragmentRandomSpread
	p.FragmentAngle = u.FragmentAngle
	if u.FragmentVelocityMin > 0 {
		p.FragmentVelocityMin = u.FragmentVelocityMin
	}
	if u.FragmentVelocityMax > 0 {
		p.FragmentVelocityMax = u.FragmentVelocityMax
	}
	if u.FragmentLifeMin > 0 {
		p.FragmentLifeMin = u.FragmentLifeMin
	}
	if u.FragmentLifeMax > 0 {
		p.FragmentLifeMax = u.FragmentLifeMax
	}
	p.FragmentBullet = convertVanillaBulletProfile(u.FragmentBullet)
	p.StatusID = u.StatusID
	p.StatusName = strings.ToLower(strings.TrimSpace(u.StatusName))
	p.StatusDuration = u.StatusDuration
	p.ShootStatusID = u.ShootStatusID
	p.ShootStatusName = strings.ToLower(strings.TrimSpace(u.ShootStatusName))
	p.ShootStatusDuration = u.ShootStatusDuration
	if strings.TrimSpace(u.ShootEffect) != "" {
		p.ShootEffect = strings.TrimSpace(u.ShootEffect)
	}
	if strings.TrimSpace(u.SmokeEffect) != "" {
		p.SmokeEffect = strings.TrimSpace(u.SmokeEffect)
	}
	if strings.TrimSpace(u.HitEffect) != "" {
		p.HitEffect = strings.TrimSpace(u.HitEffect)
	}
	if strings.TrimSpace(u.DespawnEffect) != "" {
		p.DespawnEffect = strings.TrimSpace(u.DespawnEffect)
	}
	p.PreferBuildings = u.PreferBuildings
	p.TargetAir = u.TargetAir
	p.TargetGround = u.TargetGround
	if strings.TrimSpace(u.TargetPriority) != "" {
		p.TargetPriority = strings.TrimSpace(u.TargetPriority)
	}
	p.HitBuildings = u.HitBuildings
}

func mergeBuildingProfile(p *buildingWeaponProfile, t vanillaTurretProfile) {
	if p == nil {
		return
	}
	if strings.TrimSpace(t.ClassName) != "" {
		p.ClassName = strings.TrimSpace(t.ClassName)
	}
	if strings.TrimSpace(t.FireMode) != "" {
		p.FireMode = strings.TrimSpace(t.FireMode)
	}
	if t.Range > 0 {
		p.Range = t.Range
	}
	if t.Damage > 0 {
		p.Damage = t.Damage
	}
	if t.SplashDamage > 0 {
		p.SplashDamage = t.SplashDamage
	}
	if t.Interval > 0 || t.ContinuousHold {
		p.Interval = t.Interval
	}
	if t.BulletType > 0 {
		p.BulletType = t.BulletType
	}
	p.BulletClass = ""
	if t.Bullet != nil {
		p.BulletClass = strings.TrimSpace(t.Bullet.ClassName)
	}
	if t.BulletSpeed > 0 {
		p.BulletSpeed = t.BulletSpeed
	}
	if t.BulletLifetime > 0 {
		p.BulletLifetime = t.BulletLifetime
	}
	if t.BulletHitSize > 0 {
		p.BulletHitSize = t.BulletHitSize
	}
	p.SplashRadius = t.SplashRadius
	p.BuildingDamage = t.BuildingDamageMultiplier
	p.ArmorMultiplier = t.ArmorMultiplier
	p.MaxDamageFraction = t.MaxDamageFraction
	p.ShieldDamageMul = t.ShieldDamageMultiplier
	p.PierceDamageFactor = t.PierceDamageFactor
	p.PierceArmor = t.PierceArmor
	p.SlowSec = t.SlowSec
	if t.SlowMul > 0 {
		p.SlowMul = t.SlowMul
	}
	p.Pierce = t.Pierce
	p.PierceBuilding = t.PierceBuilding
	p.ChainCount = t.ChainCount
	p.ChainRange = t.ChainRange
	p.StatusID = t.StatusID
	p.StatusName = strings.ToLower(strings.TrimSpace(t.StatusName))
	p.StatusDuration = t.StatusDuration
	p.FragmentCount = t.FragmentCount
	p.FragmentSpread = t.FragmentSpread
	p.FragmentSpeed = t.FragmentSpeed
	p.FragmentLife = t.FragmentLife
	p.FragmentRandomSpread = t.FragmentRandomSpread
	p.FragmentAngle = t.FragmentAngle
	if t.FragmentVelocityMin > 0 {
		p.FragmentVelocityMin = t.FragmentVelocityMin
	}
	if t.FragmentVelocityMax > 0 {
		p.FragmentVelocityMax = t.FragmentVelocityMax
	}
	if t.FragmentLifeMin > 0 {
		p.FragmentLifeMin = t.FragmentLifeMin
	}
	if t.FragmentLifeMax > 0 {
		p.FragmentLifeMax = t.FragmentLifeMax
	}
	p.FragmentBullet = convertVanillaBulletProfile(t.FragmentBullet)
	p.Bullet = convertVanillaBulletProfile(t.Bullet)
	if strings.TrimSpace(t.ShootEffect) != "" {
		p.ShootEffect = strings.TrimSpace(t.ShootEffect)
	}
	if strings.TrimSpace(t.SmokeEffect) != "" {
		p.SmokeEffect = strings.TrimSpace(t.SmokeEffect)
	}
	if strings.TrimSpace(t.HitEffect) != "" {
		p.HitEffect = strings.TrimSpace(t.HitEffect)
	}
	if strings.TrimSpace(t.DespawnEffect) != "" {
		p.DespawnEffect = strings.TrimSpace(t.DespawnEffect)
	}
	p.HitBuildings = t.HitBuildings
	p.TargetBuilds = t.TargetBuilds
	p.TargetAir = t.TargetAir
	p.TargetGround = t.TargetGround
	p.Rotate = t.Rotate
	if t.RotateSpeed > 0 {
		p.RotateSpeed = t.RotateSpeed
	}
	p.BaseRotation = t.BaseRotation
	p.PredictTarget = t.PredictTarget
	if t.TargetInterval > 0 {
		p.TargetInterval = t.TargetInterval
	}
	if t.TargetSwitchInterval > 0 {
		p.TargetSwitchInterval = t.TargetSwitchInterval
	}
	if t.ShootCone > 0 {
		p.ShootCone = t.ShootCone
	}
	if t.RotationLimit > 0 {
		p.RotationLimit = t.RotationLimit
	}
	if strings.TrimSpace(t.TargetPriority) != "" {
		p.TargetPriority = strings.TrimSpace(t.TargetPriority)
	}
	p.AmmoCapacity = t.AmmoCapacity
	p.AmmoRegen = t.AmmoRegen
	p.AmmoPerShot = t.AmmoPerShot
	p.PowerCapacity = t.PowerCapacity
	p.PowerRegen = t.PowerRegen
	p.PowerPerShot = t.PowerPerShot
	p.BurstShots = t.BurstShots
	p.BurstSpacing = t.BurstSpacing
	p.ContinuousHold = t.ContinuousHold
	p.AimChangeSpeed = t.AimChangeSpeed
	p.ShootDuration = t.ShootDuration
}

func (w *World) stepEntities(delta time.Duration) (movementDur, combatDur, buildingCombatDur, bulletDur time.Duration) {
	if w.model == nil {
		return 0, 0, 0, 0
	}
	dt := float32(delta.Seconds())
	if dt <= 0 {
		return 0, 0, 0, 0
	}
	movementStartedAt := time.Now()
	maxX := float32(w.model.Width * 8)
	maxY := float32(w.model.Height * 8)
	idToIndex := map[int32]int{}
	for i := range w.model.Entities {
		w.ensureEntityDefaults(&w.model.Entities[i])
		idToIndex[w.model.Entities[i].ID] = i
	}
	spatial := buildEntitySpatialIndex(w.model.Entities)
	teamSpatial := buildTeamEntitySpatialIndexes(w.model.Entities)
	for i := 0; i < len(w.model.Entities); {
		e := &w.model.Entities[i]
		changed := false
		if e.SlowRemain > 0 {
			e.SlowRemain -= dt
			if e.SlowRemain <= 0 {
				e.SlowRemain = 0
				e.SlowMul = 1
			}
			changed = true
		}
		prevStatusCount := len(e.Statuses)
		prevHealth := e.Health
		prevShield := e.Shield
		prevDisarmed := e.Disarmed
		w.updateEntityStatuses(e, dt)
		if prevStatusCount != len(e.Statuses) || prevHealth != e.Health || prevShield != e.Shield || prevDisarmed != e.Disarmed {
			changed = true
		}
		if e.Shield < e.ShieldMax && e.ShieldRegen > 0 {
			e.Shield += e.ShieldRegen * dt
			if e.Shield > e.ShieldMax {
				e.Shield = e.ShieldMax
			}
			changed = true
		}
		w.stepEntityAutonomousAILocked(e, dt, spatial, teamSpatial)
		applyBehaviorMotion(e, w.model.Entities, idToIndex)
		if e.VelX != 0 || e.VelY != 0 {
			e.X += e.VelX * dt
			e.Y += e.VelY * dt
			changed = true
		}
		if e.RotVel != 0 {
			e.Rotation += e.RotVel * dt
			changed = true
		}
		if e.LifeSec > 0 {
			e.AgeSec += dt
			changed = true
		}
		if w.stepEntityMiningLocked(e, dt) {
			changed = true
		}
		if changed {
			w.model.EntitiesRev++
		}

		out := e.X < 0 || e.Y < 0 || e.X > maxX || e.Y > maxY
		if out && e.PlayerID != 0 {
			e.X = clampf(e.X, 0, maxX)
			e.Y = clampf(e.Y, 0, maxY)
			e.VelX = 0
			e.VelY = 0
			out = false
			w.model.EntitiesRev++
		}
		expired := e.LifeSec > 0 && e.AgeSec >= e.LifeSec
		dead := e.Health <= 0
		if !out && !expired && !dead {
			i++
			continue
		}
		removed := *e
		if dead {
			w.handleEntityDeathAbilitiesLocked(removed)
		}
		delete(w.builderStates, removed.ID)
		w.cancelBuildPlansByOwnerLocked(removed.ID)
		delete(w.unitMountCDs, removed.ID)
		delete(w.unitMountStates, removed.ID)
		delete(w.unitTargets, removed.ID)
		delete(w.unitAIStates, removed.ID)
		delete(w.unitMiningStates, removed.ID)
		last := len(w.model.Entities) - 1
		w.model.Entities[i] = w.model.Entities[last]
		w.model.Entities = w.model.Entities[:last]
		w.model.EntitiesRev++
		w.entityEvents = append(w.entityEvents, EntityEvent{
			Kind:   EntityEventRemoved,
			Entity: removed,
		})
	}

	idToIndex = map[int32]int{}
	for i := range w.model.Entities {
		idToIndex[w.model.Entities[i].ID] = i
	}
	spatial = buildEntitySpatialIndex(w.model.Entities)
	teamSpatial = buildTeamEntitySpatialIndexes(w.model.Entities)
	movementDur = time.Since(movementStartedAt)

	w.stepEntityAbilities(dt)
	idToIndex = map[int32]int{}
	for i := range w.model.Entities {
		idToIndex[w.model.Entities[i].ID] = i
	}
	spatial = buildEntitySpatialIndex(w.model.Entities)
	teamSpatial = buildTeamEntitySpatialIndexes(w.model.Entities)

	combatStartedAt := time.Now()
	w.stepEntityCombat(dt, idToIndex, spatial, teamSpatial)
	combatDur = time.Since(combatStartedAt)

	buildingCombatStartedAt := time.Now()
	w.stepBuildingCombat(dt, idToIndex, spatial, teamSpatial)
	buildingCombatDur = time.Since(buildingCombatStartedAt)

	bulletStartedAt := time.Now()
	w.stepPendingMountShots(dt, idToIndex)
	w.stepBullets(dt, idToIndex, spatial, teamSpatial)
	bulletDur = time.Since(bulletStartedAt)
	return movementDur, combatDur, buildingCombatDur, bulletDur
}

func (w *World) stepEntityCombat(dt float32, idToIndex map[int32]int, spatial *entitySpatialIndex, teamSpatial map[TeamID]*entitySpatialIndex) {
	ents := w.model.Entities
	if len(ents) == 0 {
		return
	}
	for i := range ents {
		e := &ents[i]
		if !canEntityAttack(*e) {
			continue
		}
		if mounts := w.unitMountProfilesForEntity(*e); len(mounts) > 0 {
			w.stepEntityMountedCombat(e, mounts, dt, idToIndex, spatial, teamSpatial)
			continue
		}
		if e.AttackCooldown > 0 {
			e.AttackCooldown -= dt * attackCooldownScale(*e)
			if e.AttackCooldown < 0 {
				e.AttackCooldown = 0
			}
			continue
		}
		rangeLimit := e.AttackRange
		if rangeLimit <= 0 {
			rangeLimit = 56
		}
		track := w.unitTargets[e.ID]
		retargetDelay := maxf(e.AttackInterval*0.45, 0.18)
		if e.AttackBuildings && e.AttackPreferBuildings {
			if pos, tx, ty, ok := w.findNearestEnemyBuilding(*e, rangeLimit); ok {
				if !w.tryConsumeEntityAmmoLocked(e, maxf(e.AmmoPerShot, 1)) {
					w.unitTargets[e.ID] = track
					continue
				}
				e.AttackCooldown = maxf(e.AttackInterval, 0.2)
				e.Rotation = lookAt(e.X, e.Y, tx, ty)
				w.applyShootStatus(e)
				if e.AttackFireMode == "beam" {
					w.fireBeamAtBuilding(*e, pos, tx, ty, false)
				} else {
					w.spawnBullet(*e, tx, ty, false)
				}
				w.unitTargets[e.ID] = track
				continue
			}
		}
		if tid, ok := w.acquireTrackedEntityTarget(*e, ents, idToIndex, spatial, teamSpatial, rangeLimit, e.AttackTargetAir, e.AttackTargetGround, e.AttackTargetPriority, &track, dt, retargetDelay); ok {
			if idx, exists := idToIndex[tid]; exists && idx >= 0 && idx < len(ents) {
				target := &ents[idx]
				if !w.tryConsumeEntityAmmoLocked(e, maxf(e.AmmoPerShot, 1)) {
					w.unitTargets[e.ID] = track
					continue
				}
				e.AttackCooldown = maxf(e.AttackInterval, 0.2)
				e.Rotation = lookAt(e.X, e.Y, target.X, target.Y)
				w.applyShootStatus(e)
				if e.AttackFireMode == "beam" {
					w.fireBeamAtEntity(*e, target, idx, false)
				} else {
					w.spawnBullet(*e, target.X, target.Y, false)
				}
				w.unitTargets[e.ID] = track
				continue
			}
		}
		w.unitTargets[e.ID] = track
		if e.AttackBuildings {
			if pos, tx, ty, ok := w.findNearestEnemyBuilding(*e, rangeLimit); ok {
				if !w.tryConsumeEntityAmmoLocked(e, maxf(e.AmmoPerShot, 1)) {
					continue
				}
				e.AttackCooldown = maxf(e.AttackInterval, 0.2)
				e.Rotation = lookAt(e.X, e.Y, tx, ty)
				w.applyShootStatus(e)
				if e.AttackFireMode == "beam" {
					w.fireBeamAtBuilding(*e, pos, tx, ty, false)
				} else {
					w.spawnBullet(*e, tx, ty, false)
				}
			}
		}
	}
}

func (w *World) stepEntityMountedCombat(e *RawEntity, mounts []unitWeaponMountProfile, dt float32, idToIndex map[int32]int, spatial *entitySpatialIndex, teamSpatial map[TeamID]*entitySpatialIndex) {
	if e == nil || len(mounts) == 0 {
		return
	}
	states := w.ensureUnitMountStates(e.ID, mounts)
	scale := attackCooldownScale(*e)
	for i := range mounts {
		lastReload := states[i].Reload
		if states[i].Reload > 0 {
			states[i].Reload -= dt * scale
			if states[i].Reload < 0 {
				states[i].Reload = 0
			}
		}
		if mounts[i].Alternate && mounts[i].OtherSide >= 0 {
			half := mounts[i].Interval * 0.5
			if half > 0 && states[i].Reload <= half && lastReload > half {
				other := int(mounts[i].OtherSide)
				states[i].Side = !states[i].Side
				if other >= 0 && other < len(states) {
					states[other].Side = !states[other].Side
				}
			}
		}
	}

	for mi := range mounts {
		mount := mounts[mi]
		state := &states[mi]
		rangeLimit := mount.Range
		if rangeLimit <= 0 {
			rangeLimit = e.AttackRange
		}
		if rangeLimit <= 0 {
			rangeLimit = 56
		}
		baseX, baseY := unitMountBasePosition(*e, mount)

		if mount.NoAttack {
			state.TargetBuildPos = -1
			state.Warmup = warmupToward(state.Warmup, 0, mount.ShootWarmupSpeed, mount.LinearWarmup, dt)
			if mount.RepairBeam {
				src := RawEntity{ID: e.ID, Team: e.Team, X: baseX, Y: baseY}
				if entIdx, pos, tx, ty, ok := w.findRepairTarget(src, mount, rangeLimit); ok {
					w.updateMountAim(*e, mount, state, tx, ty, dt)
					state.Warmup = warmupToward(state.Warmup, 1, mount.ShootWarmupSpeed, mount.LinearWarmup, dt)
					deltaFrames := dt * 60
					if entIdx >= 0 && entIdx < len(w.model.Entities) {
						target := &w.model.Entities[entIdx]
						amount := mount.RepairSpeed*deltaFrames + mount.FractionRepairSpeed*deltaFrames*target.MaxHealth/100
						_ = w.healEntity(target, amount)
					} else if pos >= 0 {
						x := int(pos) % w.model.Width
						y := int(pos) / w.model.Width
						if w.model.InBounds(x, y) {
							build := w.model.Tiles[pos].Build
							if build != nil {
								amount := mount.RepairSpeed*deltaFrames + mount.FractionRepairSpeed*deltaFrames*build.MaxHealth/100
								_ = w.healBuilding(pos, amount)
							}
						}
					}
				}
			}
			continue
		}

		if mount.PointDefense {
			state.TargetBuildPos = -1
			targetIdx := w.findPointDefenseTarget(e.Team, baseX, baseY, rangeLimit)
			state.Warmup = warmupToward(state.Warmup, 0, mount.ShootWarmupSpeed, mount.LinearWarmup, dt)
			if targetIdx >= 0 {
				target := w.bullets[targetIdx]
				w.updateMountAim(*e, mount, state, target.X, target.Y, dt)
				state.Warmup = warmupToward(state.Warmup, 1, mount.ShootWarmupSpeed, mount.LinearWarmup, dt)
				if states[mi].Reload <= 0 &&
					(mount.MinShootVelocity < 0 || entityVelocityLen(*e) >= mount.MinShootVelocity) &&
					state.Warmup >= mount.MinWarmup &&
					(!mount.Rotate || angleWithin(state.Rotation, state.TargetRotation, mount.ShootCone)) &&
					(!mount.Alternate || mount.OtherSide < 0 || state.Side == mount.FlipSprite) {
					src := *e
					applyMountWeaponProfile(&src, mount)
					sx, sy, _ := unitMountShootPosition(*e, mount, *state)
					src.X = sx
					src.Y = sy
					src.Rotation = lookAt(sx, sy, target.X, target.Y)
					if w.firePointDefenseMount(src, targetIdx) {
						reload := mount.Interval
						if reload <= 0 {
							reload = 1.0 / 60.0
						}
						state.Reload = reload
					}
				}
			}
			continue
		}

		if mount.MinShootVelocity >= 0 && entityVelocityLen(*e) < mount.MinShootVelocity {
			continue
		}

		src := RawEntity{ID: e.ID, Team: e.Team, X: baseX, Y: baseY}
		track := targetTrackState{TargetID: state.TargetID, RetargetCD: state.RetargetCD}
		retargetDelay := mount.TargetInterval
		if track.TargetID != 0 && mount.TargetSwitchInterval > 0 {
			retargetDelay = mount.TargetSwitchInterval
		}

		unitIdx := -1
		targetBuild := false
		buildPos := int32(-1)
		targetX, targetY := float32(0), float32(0)

		if mount.PreferBuildings && mount.HitBuildings {
			if pos, tx, ty, ok := w.findNearestEnemyBuilding(src, rangeLimit); ok {
				targetBuild = true
				buildPos = pos
				targetX, targetY = tx, ty
			}
		}

		if !targetBuild {
			if tid, ok := w.acquireTrackedEntityTarget(src, w.model.Entities, idToIndex, spatial, teamSpatial, rangeLimit, mount.TargetAir, mount.TargetGround, "nearest", &track, dt, retargetDelay); ok {
				if idx, exists := idToIndex[tid]; exists && idx >= 0 && idx < len(w.model.Entities) {
					unitIdx = idx
					targetX = w.model.Entities[idx].X
					targetY = w.model.Entities[idx].Y
				}
			}
		}

		if unitIdx < 0 && mount.HitBuildings {
			if pos, tx, ty, ok := w.findNearestEnemyBuilding(src, rangeLimit); ok {
				targetBuild = true
				buildPos = pos
				targetX, targetY = tx, ty
			}
		}

		state.TargetID = track.TargetID
		state.RetargetCD = track.RetargetCD
		state.TargetBuildPos = buildPos
		beamActive := mount.Continuous && state.BeamBulletID != 0
		reload := mount.Interval
		if reload <= 0 {
			reload = maxf(e.AttackInterval, 1.0/60.0)
		}
		warmupTarget := float32(0)
		if beamActive {
			warmupTarget = 1
		}
		if unitIdx < 0 && !targetBuild {
			state.Warmup = warmupToward(state.Warmup, warmupTarget, mount.ShootWarmupSpeed, mount.LinearWarmup, dt)
			if beamActive {
				if w.updateMountedBeamBullet(e, mount, state, dt) {
					state.Reload = reload
					continue
				}
			}
			continue
		}
		warmupTarget = 1

		w.updateMountAim(*e, mount, state, targetX, targetY, dt)
		state.Warmup = warmupToward(state.Warmup, warmupTarget, mount.ShootWarmupSpeed, mount.LinearWarmup, dt)
		if beamActive {
			if w.updateMountedBeamBullet(e, mount, state, dt) {
				if mount.AlwaysContinuous {
					w.keepMountedBeamAlive(e, mount, state)
				}
				state.Reload = reload
				continue
			}
		}
		if state.Reload > 0 && !(mount.AlwaysContinuous && state.BeamBulletID == 0) {
			continue
		}
		if state.Warmup < mount.MinWarmup {
			continue
		}
		if mount.Alternate && mount.OtherSide >= 0 && state.Side != mount.FlipSprite {
			continue
		}
		if mount.Rotate {
			if !angleWithin(state.Rotation, state.TargetRotation, mount.ShootCone) {
				continue
			}
		} else if !mount.AlwaysShooting && !angleWithin(e.Rotation+mount.BaseRotation, state.TargetRotation, mount.ShootCone) {
			continue
		}

		if w.triggerEntityMountFire(e, mi, mount, state, idToIndex) {
			state.Reload = reload
		}
	}

	w.unitMountStates[e.ID] = states
}

func (w *World) fireEntityMountAtUnit(e *RawEntity, target *RawEntity, mount unitWeaponMountProfile, state unitMountState, targetIdx int) bool {
	if e == nil || target == nil || target.Health <= 0 {
		return false
	}
	src := *e
	applyMountWeaponProfile(&src, mount)
	sx, sy, _ := unitMountShootPosition(*e, mount, state)
	src.X = sx
	src.Y = sy
	src.Rotation = lookAt(sx, sy, target.X, target.Y)
	if src.AttackFireMode == "beam" {
		w.applyMountShootStatus(e, mount)
		w.fireBeamAtEntity(src, target, targetIdx, false)
		return true
	}
	w.applyMountShootStatus(e, mount)
	w.spawnBullet(src, target.X, target.Y, false)
	return true
}

func (w *World) fireEntityMountAtBuilding(e *RawEntity, pos int32, tx, ty float32, mount unitWeaponMountProfile, state unitMountState) bool {
	if e == nil {
		return false
	}
	src := *e
	applyMountWeaponProfile(&src, mount)
	sx, sy, _ := unitMountShootPosition(*e, mount, state)
	src.X = sx
	src.Y = sy
	src.Rotation = lookAt(sx, sy, tx, ty)
	if src.AttackFireMode == "beam" {
		w.applyMountShootStatus(e, mount)
		w.fireBeamAtBuilding(src, pos, tx, ty, false)
		return true
	}
	w.applyMountShootStatus(e, mount)
	w.spawnBullet(src, tx, ty, false)
	return true
}

func applyMountStats(src *RawEntity, mount unitWeaponMountProfile) {
	if src == nil {
		return
	}
	if mount.DamageMul > 0 {
		src.AttackDamage *= mount.DamageMul
		src.AttackSplashDamage *= mount.DamageMul
	}
	if mount.RangeMul > 0 {
		src.AttackRange *= mount.RangeMul
	}
	if mount.BulletSpeedMul > 0 {
		src.AttackBulletSpeed *= mount.BulletSpeedMul
	}
	if mount.SplashRadiusMul > 0 {
		src.AttackSplashRadius *= mount.SplashRadiusMul
	}
	if mount.BulletType >= 0 {
		src.AttackBulletType = mount.BulletType
	}
}

func (w *World) stepBuildingCombat(dt float32, idToIndex map[int32]int, spatial *entitySpatialIndex, teamSpatial map[TeamID]*entitySpatialIndex) {
	if w.model == nil {
		return
	}
	ents := w.model.Entities
	for _, pos := range w.turretTilePositions {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			continue
		}
		t := &w.model.Tiles[pos]
		if t.Build == nil || t.Build.Health <= 0 || t.Build.Team == 0 {
			continue
		}
		prof, ok := w.getBuildingWeaponProfile(int16(t.Build.Block))
		if !ok || (prof.Damage <= 0 && prof.SplashDamage <= 0 && prof.StatusID == 0 && strings.TrimSpace(prof.StatusName) == "") || (!prof.ContinuousHold && prof.Interval <= 0) || prof.Range <= 0 {
			continue
		}
		prof = w.resolveBuildingWeaponProfileLocked(t, prof)
		state, exists := w.buildStates[pos]
		if !exists {
			state = buildCombatState{
				Ammo:           prof.AmmoCapacity,
				Power:          prof.PowerCapacity,
				TurretRotation: float32(t.Rotation) * 90,
				HasRotation:    true,
			}
		} else if !state.HasRotation {
			state.TurretRotation = float32(t.Rotation) * 90
			state.HasRotation = true
		}
		scaledDT := dt * w.buildingTimeScaleLocked(pos)
		state = w.regenBuildState(pos, t, state, prof, t.Build.Team, scaledDT)
		if state.Cooldown > 0 {
			state.Cooldown -= scaledDT
			if state.Cooldown < 0 {
				state.Cooldown = 0
			}
		}
		if state.BurstDelay > 0 {
			state.BurstDelay -= scaledDT
			if state.BurstDelay < 0 {
				state.BurstDelay = 0
			}
		}
		if state.RetargetCD > 0 {
			state.RetargetCD -= scaledDT
			if state.RetargetCD < 0 {
				state.RetargetCD = 0
			}
		}
		if state.BuildTargetCD > 0 {
			state.BuildTargetCD -= scaledDT
			if state.BuildTargetCD < 0 {
				state.BuildTargetCD = 0
			}
		}

		src := RawEntity{
			X:                        float32(t.X*8 + 4),
			Y:                        float32(t.Y*8 + 4),
			Rotation:                 float32(t.Rotation) * 90,
			Team:                     t.Build.Team,
			AttackFireMode:           prof.FireMode,
			AttackDamage:             prof.Damage,
			AttackSplashDamage:       prof.SplashDamage,
			AttackInterval:           prof.Interval,
			AttackRange:              prof.Range,
			AttackBulletType:         prof.BulletType,
			AttackBulletLifetime:     prof.BulletLifetime,
			AttackBulletHitSize:      prof.BulletHitSize,
			AttackBulletSpeed:        prof.BulletSpeed,
			AttackSplashRadius:       prof.SplashRadius,
			AttackBuildingDamage:     defaultBuildingDamageMultiplier(prof.BuildingDamage, prof.HitBuildings),
			AttackBuildingDamageSet:  prof.HitBuildings || prof.BuildingDamage != 0,
			AttackArmorMultiplier:    prof.ArmorMultiplier,
			AttackMaxDamageFraction:  prof.MaxDamageFraction,
			AttackShieldDamageMul:    prof.ShieldDamageMul,
			AttackPierceDamageFactor: prof.PierceDamageFactor,
			AttackPierceArmor:        prof.PierceArmor,
			AttackSlowSec:            prof.SlowSec,
			AttackSlowMul:            prof.SlowMul,
			AttackPierce:             prof.Pierce,
			AttackPierceBuilding:     prof.PierceBuilding,
			AttackChainCount:         prof.ChainCount,
			AttackChainRange:         prof.ChainRange,
			AttackStatusID:           prof.StatusID,
			AttackStatusName:         prof.StatusName,
			AttackStatusDuration:     prof.StatusDuration,
			AttackFragmentCount:      prof.FragmentCount,
			AttackFragmentSpread:     prof.FragmentSpread,
			AttackFragmentSpeed:      prof.FragmentSpeed,
			AttackFragmentLife:       prof.FragmentLife,
			AttackFragmentRand:       prof.FragmentRandomSpread,
			AttackFragmentAngle:      prof.FragmentAngle,
			AttackFragmentVelMin:     prof.FragmentVelocityMin,
			AttackFragmentVelMax:     prof.FragmentVelocityMax,
			AttackFragmentLifeMin:    prof.FragmentLifeMin,
			AttackFragmentLifeMax:    prof.FragmentLifeMax,
			AttackFragmentBullet:     cloneBulletRuntimeProfile(prof.FragmentBullet),
			AttackShootEffect:        prof.ShootEffect,
			AttackSmokeEffect:        prof.SmokeEffect,
			AttackHitEffect:          prof.HitEffect,
			AttackDespawnEffect:      prof.DespawnEffect,
			AttackTargetAir:          prof.TargetAir,
			AttackTargetGround:       prof.TargetGround,
			AttackTargetPriority:     prof.TargetPriority,
			AttackBuildings:          prof.HitBuildings,
		}
		if state.HasRotation {
			src.Rotation = state.TurretRotation
		}

		controlled, controlledCanShoot, aimX, aimY := w.controlledBuildingAimLocked(pos)
		targetIdx := -1
		targetBuildPos := int32(-1)
		targetX, targetY := float32(0), float32(0)
		hasAim := false
		targetRotation := src.Rotation
		if controlled {
			targetX, targetY = aimX, aimY
			hasAim = true
			targetRotation = updateBuildingAim(t, src, prof, &state, targetX, targetY, scaledDT)
			src.Rotation = state.TurretRotation
		} else {
			targetIdx, targetBuildPos, targetX, targetY = w.acquireBuildingWeaponTarget(src, &state, prof, ents, idToIndex, spatial, teamSpatial)
			hasAim = targetIdx >= 0 || targetBuildPos >= 0
			if targetIdx >= 0 && targetIdx < len(ents) {
				targetX, targetY = predictBuildingAimPosition(src, ents[targetIdx], prof)
			}
			if hasAim {
				targetRotation = updateBuildingAim(t, src, prof, &state, targetX, targetY, scaledDT)
				src.Rotation = state.TurretRotation
			}
		}

		if prof.ContinuousHold && isPersistentBeamBulletProfile(prof.Bullet) {
			if w.stepBuildingContinuousBeam(pos, &src, &state, prof, ents, idToIndex, spatial, teamSpatial, scaledDT) {
				state.TurretRotation = src.Rotation
				state.HasRotation = true
			}
			w.buildStates[pos] = state
			continue
		}

		allowShot := hasAim && state.Cooldown <= 0 && (state.BurstRemain == 0 || state.BurstDelay <= 0)
		if allowShot && controlled && !controlledCanShoot {
			allowShot = false
		}
		if allowShot && !buildingCanFireAtAim(prof, src.Rotation, targetRotation) {
			allowShot = false
		}
		if allowShot && w.tryFireBuildingShot(pos, t, &src, &state, prof, ents, targetIdx, targetBuildPos, targetX, targetY, controlled, controlledCanShoot) {
			if state.BurstRemain > 0 {
				state.BurstRemain--
				state.BurstDelay = maxf(prof.BurstSpacing, 0.02)
			} else {
				shots := prof.BurstShots
				if shots < 1 {
					shots = 1
				}
				state.BurstRemain = shots - 1
				if state.BurstRemain > 0 {
					state.BurstDelay = maxf(prof.BurstSpacing, 0.02)
				}
				state.Cooldown = maxf(prof.Interval, 0.05)
			}
			state.TurretRotation = src.Rotation
			state.HasRotation = true
		}
		w.buildStates[pos] = state
	}
}

func (w *World) buildingUsesItemAmmoLocked(tile *Tile, prof buildingWeaponProfile) bool {
	if w == nil || tile == nil || tile.Build == nil {
		return false
	}
	name := w.blockNameByID(int16(tile.Build.Block))
	return classifyTurretBlockSyncKind(name, prof) == blockSyncItemTurret
}

func (w *World) buildingWeaponProfileByNameLocked(name string) (buildingWeaponProfile, bool) {
	if w == nil || name == "" {
		return buildingWeaponProfile{}, false
	}
	src := w.buildingProfilesByName
	if len(src) == 0 {
		src = buildingWeaponProfilesByName
	}
	if prof, ok := src[name]; ok {
		return prof, true
	}
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" || normalized == name {
		return buildingWeaponProfile{}, false
	}
	prof, ok := src[normalized]
	return prof, ok
}

func (w *World) buildingHidesInventoryItemsLocked(pos int32, tile *Tile) bool {
	_ = pos
	if w == nil || tile == nil || tile.Build == nil {
		return false
	}
	prof, ok := w.getBuildingWeaponProfile(int16(tile.Build.Block))
	return ok && w.buildingUsesItemAmmoLocked(tile, prof)
}

func (w *World) resolveBuildingWeaponProfileLocked(tile *Tile, prof buildingWeaponProfile) buildingWeaponProfile {
	if w == nil || tile == nil || tile.Build == nil {
		return prof
	}
	name := w.blockNameByID(int16(tile.Build.Block))
	if ammoByItem, ok := turretItemAmmoBulletTypesByName[name]; ok {
		if ammoItem, ok := w.currentBuildingAmmoItemLocked(tile, prof); ok {
			if bulletType, exists := ammoByItem[ammoItem]; exists && bulletType > 0 {
				prof.BulletType = bulletType
				return prof
			}
		}
	}
	if ammoByLiquid, ok := turretLiquidAmmoBulletTypesByName[name]; ok {
		bestAmount := float32(0)
		bestType := int16(0)
		for _, stack := range tile.Build.Liquids {
			if stack.Amount <= 0.0001 {
				continue
			}
			if bulletType, exists := ammoByLiquid[stack.Liquid]; exists && bulletType > 0 && stack.Amount > bestAmount {
				bestAmount = stack.Amount
				bestType = bulletType
			}
		}
		if bestType > 0 {
			prof.BulletType = bestType
		}
	}
	return prof
}

func (w *World) buildingAcceptsAmmoItemLocked(tile *Tile, prof buildingWeaponProfile, item ItemID) bool {
	if w == nil || tile == nil || tile.Build == nil {
		return false
	}
	name := w.blockNameByID(int16(tile.Build.Block))
	ammoByItem, ok := turretItemAmmoBulletTypesByName[name]
	if !ok {
		return true
	}
	_, exists := ammoByItem[item]
	return exists
}

func (w *World) firstBuildingAmmoItemLocked(tile *Tile, prof buildingWeaponProfile) (ItemID, bool) {
	if tile == nil || tile.Build == nil {
		return 0, false
	}
	w.normalizeTurretAmmoEntriesLocked(tile, prof)
	for _, entry := range tile.Build.Items {
		if entry.Amount <= 0 || !w.buildingAcceptsAmmoItemLocked(tile, prof, entry.Item) {
			continue
		}
		return entry.Item, true
	}
	return 0, false
}

func (w *World) buildingItemAmmoCapacityLocked(tile *Tile, prof buildingWeaponProfile) int32 {
	if !w.buildingUsesItemAmmoLocked(tile, prof) || prof.AmmoCapacity <= 0 {
		return 0
	}
	return int32(math.Ceil(float64(prof.AmmoCapacity)))
}

func buildingAmmoPerShotCount(prof buildingWeaponProfile) int32 {
	if prof.AmmoPerShot <= 0 {
		return 0
	}
	amount := int32(math.Ceil(float64(prof.AmmoPerShot)))
	if amount < 1 {
		amount = 1
	}
	return amount
}

func (w *World) buildingHasAmmoLocked(pos int32, tile *Tile, prof buildingWeaponProfile, state buildCombatState) bool {
	if w.buildingUsesItemAmmoLocked(tile, prof) {
		required := buildingAmmoPerShotCount(prof)
		if required <= 0 || tile == nil || tile.Build == nil {
			return required <= 0
		}
		if amount, ok := w.currentBuildingAmmoAmountLocked(tile, prof); ok {
			return amount >= required
		}
		return false
	}
	if prof.AmmoPerShot > 0 {
		return state.Ammo >= prof.AmmoPerShot
	}
	return true
}

func (w *World) consumeBuildingAmmoLocked(pos int32, tile *Tile, prof buildingWeaponProfile, state *buildCombatState) bool {
	if prof.AmmoPerShot <= 0 {
		return true
	}
	if w.buildingUsesItemAmmoLocked(tile, prof) {
		if tile == nil || tile.Build == nil {
			return false
		}
		remaining := buildingAmmoPerShotCount(prof)
		index := w.currentBuildingAmmoIndexLocked(tile, prof)
		if index < 0 || index >= len(tile.Build.Items) || tile.Build.Items[index].Amount < remaining {
			return false
		}
		tile.Build.Items[index].Amount -= remaining
		if tile.Build.Items[index].Amount <= 0 {
			tile.Build.Items = append(tile.Build.Items[:index], tile.Build.Items[index+1:]...)
		}
		w.emitBlockItemSyncLocked(pos)
		return true
	}
	if state == nil || state.Ammo < prof.AmmoPerShot {
		return false
	}
	state.Ammo -= prof.AmmoPerShot
	if state.Ammo < 0 {
		state.Ammo = 0
	}
	return true
}

func (w *World) regenBuildState(pos int32, tile *Tile, state buildCombatState, prof buildingWeaponProfile, team TeamID, dt float32) buildCombatState {
	if prof.AmmoCapacity > 0 && !w.buildingUsesItemAmmoLocked(tile, prof) {
		if prof.AmmoRegen > 0 {
			state.Ammo = minf(prof.AmmoCapacity, state.Ammo+prof.AmmoRegen*dt)
		}
	}
	if prof.PowerCapacity > 0 {
		if prof.PowerRegen > 0 {
			got := w.consumePowerAtLocked(pos, team, prof.PowerRegen*dt)
			state.Power = minf(prof.PowerCapacity, state.Power+got)
		}
	}
	return state
}

func buildingWeaponRetargetDelay(prof buildingWeaponProfile, hasTarget bool) float32 {
	if hasTarget && prof.TargetSwitchInterval > 0 {
		return maxf(prof.TargetSwitchInterval, 1.0/60.0)
	}
	if prof.TargetInterval > 0 {
		return maxf(prof.TargetInterval, 1.0/60.0)
	}
	return maxf(prof.Interval*0.55, 0.22)
}

func buildingWeaponShootCone(prof buildingWeaponProfile) float32 {
	if prof.ShootCone > 0 {
		return prof.ShootCone
	}
	return 5
}

func buildingCanFireAtAim(prof buildingWeaponProfile, currentRotation, targetRotation float32) bool {
	return angleWithin(currentRotation, targetRotation, buildingWeaponShootCone(prof))
}

func predictBuildingAimPosition(src RawEntity, target RawEntity, prof buildingWeaponProfile) (float32, float32) {
	if !prof.PredictTarget || prof.BulletSpeed < 0.01 {
		return target.X, target.Y
	}
	dx := target.X - src.X
	dy := target.Y - src.Y
	vx := target.VelX
	vy := target.VelY
	speed := prof.BulletSpeed
	a := vx*vx + vy*vy - speed*speed
	b := 2 * (dx*vx + dy*vy)
	c := dx*dx + dy*dy
	t := float32(-1)
	if math.Abs(float64(a)) < 1e-6 {
		if math.Abs(float64(b)) > 1e-6 {
			t = -c / b
		}
	} else {
		discriminant := b*b - 4*a*c
		if discriminant >= 0 {
			sqrtDisc := float32(math.Sqrt(float64(discriminant)))
			t1 := (-b - sqrtDisc) / (2 * a)
			t2 := (-b + sqrtDisc) / (2 * a)
			switch {
			case t1 > 0 && t2 > 0:
				t = minf(t1, t2)
			case t1 > 0:
				t = t1
			case t2 > 0:
				t = t2
			}
		}
	}
	if t <= 0 {
		return target.X, target.Y
	}
	return target.X + vx*t, target.Y + vy*t
}

func updateBuildingAim(tile *Tile, src RawEntity, prof buildingWeaponProfile, state *buildCombatState, aimX, aimY, dt float32) float32 {
	targetRotation := normalizeAngleDeg(lookAt(src.X, src.Y, aimX, aimY))
	if state == nil {
		return targetRotation
	}
	baseRotation := prof.BaseRotation
	if tile != nil {
		baseRotation += float32(tile.Rotation) * 90
	}
	currentRotation := baseRotation
	if state.HasRotation {
		currentRotation = state.TurretRotation
	}
	if prof.Rotate {
		speed := prof.RotateSpeed
		if speed <= 0 {
			speed = 20
		}
		currentRotation = moveAngleToward(currentRotation, targetRotation, speed*dt*60)
		if prof.RotationLimit > 0 && prof.RotationLimit < 360 {
			dst := angleDistDeg(currentRotation, baseRotation)
			limit := prof.RotationLimit * 0.5
			if dst > limit {
				currentRotation = moveAngleToward(currentRotation, baseRotation, dst-limit)
			}
		}
	} else {
		currentRotation = targetRotation
	}
	state.TurretRotation = normalizeAngleDeg(currentRotation)
	state.HasRotation = true
	if tile != nil {
		cardinal := buildRotationFromDegrees(state.TurretRotation)
		tile.Rotation = cardinal
		if tile.Build != nil {
			tile.Build.Rotation = cardinal
		}
	}
	return targetRotation
}

func (w *World) fireAimedBuildingBeam(src RawEntity, prof buildingWeaponProfile, tx, ty float32, sourceIsBuilding bool) bool {
	if w == nil {
		return false
	}
	beam := simBullet{
		Team:               src.Team,
		X:                  src.X,
		Y:                  src.Y,
		Damage:             src.AttackDamage * w.outgoingDamageScale(src, sourceIsBuilding),
		HitBuilds:          src.AttackBuildings,
		BuildingDamage:     entityBuildingDamageMultiplier(src),
		ArmorMultiplier:    src.AttackArmorMultiplier,
		MaxDamageFraction:  src.AttackMaxDamageFraction,
		ShieldDamageMul:    src.AttackShieldDamageMul,
		PierceDamageFactor: src.AttackPierceDamageFactor,
		PierceArmor:        src.AttackPierceArmor,
		SlowSec:            src.AttackSlowSec,
		SlowMul:            clampf(src.AttackSlowMul, 0.2, 1),
		StatusID:           src.AttackStatusID,
		StatusName:         src.AttackStatusName,
		StatusDuration:     src.AttackStatusDuration,
		TargetAir:          src.AttackTargetAir,
		TargetGround:       src.AttackTargetGround,
		AimX:               tx,
		AimY:               ty,
		BulletClass:        prof.BulletClass,
		BeamLength:         prof.Range,
		SplashRadius:       src.AttackSplashRadius,
	}
	w.emitAttackFireEffectsLocked(src)
	impacted := false
	if isPointLaserBulletClass(beam.BulletClass) {
		impacted = w.applyPointBeamDamage(beam)
	} else {
		impacted = w.applyLineBeamDamage(beam)
	}
	if impacted {
		w.emitAttackHitEffectLocked(src, tx, ty)
	}
	return true
}

func (w *World) tryFireBuildingShot(buildPos int32, tile *Tile, src *RawEntity, state *buildCombatState, prof buildingWeaponProfile, ents []RawEntity, targetIdx int, targetBuildPos int32, tx, ty float32, controlled bool, canShoot bool) bool {
	if src == nil || state == nil {
		return false
	}
	if !w.buildingHasAmmoLocked(buildPos, tile, prof, *state) {
		return false
	}
	if prof.PowerPerShot > 0 && state.Power < prof.PowerPerShot {
		return false
	}

	fired := false
	if controlled {
		if !canShoot {
			return false
		}
		if src.AttackFireMode == "beam" {
			return w.fireAimedBuildingBeam(*src, prof, tx, ty, true)
		}
		w.spawnBullet(*src, tx, ty, true)
		fired = true
	} else {
		if targetIdx >= 0 && targetIdx < len(ents) {
			target := &ents[targetIdx]
			if src.AttackFireMode == "beam" {
				w.fireBeamAtEntity(*src, target, targetIdx, true)
			} else {
				w.spawnBullet(*src, tx, ty, true)
			}
			fired = true
		}
		if !fired && targetBuildPos >= 0 {
			if src.AttackFireMode == "beam" {
				w.fireBeamAtBuilding(*src, targetBuildPos, tx, ty, true)
			} else {
				w.spawnBullet(*src, tx, ty, true)
			}
			fired = true
		}
	}
	if !fired {
		return false
	}
	if !w.consumeBuildingAmmoLocked(buildPos, tile, prof, state) {
		return false
	}
	if prof.PowerPerShot > 0 {
		state.Power -= prof.PowerPerShot
		if state.Power < 0 {
			state.Power = 0
		}
	}
	return true
}

func (w *World) spawnBullet(src RawEntity, tx, ty float32, sourceIsBuilding bool) {
	w.spawnBulletWithAngle(src, tx, ty, lookAt(src.X, src.Y, tx, ty), 1, pendingMountShot{}, sourceIsBuilding)
}

func (w *World) spawnBulletWithAngle(src RawEntity, tx, ty, angle, speedScale float32, shot pendingMountShot, sourceIsBuilding bool) {
	bulletSpeed := src.AttackBulletSpeed
	if bulletSpeed <= 0 {
		speed := src.MoveSpeed
		if speed <= 0 {
			speed = 18
		}
		bulletSpeed = maxf(speed*2.2, 28)
	}
	if speedScale <= 0 {
		speedScale = 1
	}
	bulletSpeed *= speedScale
	rad := float32(angle * math.Pi / 180)
	damageScale := w.outgoingDamageScale(src, sourceIsBuilding)
	lifeSec := src.AttackBulletLifetime
	if lifeSec <= 0 && bulletSpeed > 0 {
		lifeSec = maxf(src.AttackRange/bulletSpeed, 0.6)
	}
	radius := maxf(src.AttackBulletHitSize*0.5, 4)
	buildingMul := entityBuildingDamageMultiplier(src)
	b := simBullet{
		ID:                 w.bulletNextID,
		Team:               src.Team,
		X:                  src.X,
		Y:                  src.Y,
		VX:                 float32(math.Cos(float64(rad))) * bulletSpeed,
		VY:                 float32(math.Sin(float64(rad))) * bulletSpeed,
		Damage:             src.AttackDamage * damageScale,
		SplashDamage:       src.AttackSplashDamage * damageScale,
		LifeSec:            lifeSec,
		AgeSec:             0,
		Radius:             radius,
		HitUnits:           true,
		HitBuilds:          src.AttackBuildings,
		BulletType:         src.AttackBulletType,
		SplashRadius:       src.AttackSplashRadius,
		BuildingDamage:     buildingMul,
		ArmorMultiplier:    src.AttackArmorMultiplier,
		MaxDamageFraction:  src.AttackMaxDamageFraction,
		ShieldDamageMul:    src.AttackShieldDamageMul,
		PierceDamageFactor: src.AttackPierceDamageFactor,
		PierceArmor:        src.AttackPierceArmor,
		SlowSec:            src.AttackSlowSec,
		SlowMul:            clampf(src.AttackSlowMul, 0.2, 1),
		PierceRemain:       src.AttackPierce,
		PierceBuilding:     src.AttackPierceBuilding,
		ChainCount:         src.AttackChainCount,
		ChainRange:         src.AttackChainRange,
		FragmentCount:      src.AttackFragmentCount,
		FragmentSpread:     src.AttackFragmentSpread,
		FragmentSpeed:      src.AttackFragmentSpeed,
		FragmentLife:       src.AttackFragmentLife,
		FragmentRand:       src.AttackFragmentRand,
		FragmentAngle:      src.AttackFragmentAngle,
		FragmentVelMin:     src.AttackFragmentVelMin,
		FragmentVelMax:     src.AttackFragmentVelMax,
		FragmentLifeMin:    src.AttackFragmentLifeMin,
		FragmentLifeMax:    src.AttackFragmentLifeMax,
		FragmentBullet:     cloneBulletRuntimeProfile(src.AttackFragmentBullet),
		StatusID:           src.AttackStatusID,
		StatusName:         src.AttackStatusName,
		StatusDuration:     src.AttackStatusDuration,
		ShootEffect:        src.AttackShootEffect,
		SmokeEffect:        src.AttackSmokeEffect,
		HitEffect:          src.AttackHitEffect,
		DespawnEffect:      src.AttackDespawnEffect,
		TargetAir:          src.AttackTargetAir,
		TargetGround:       src.AttackTargetGround,
		TargetPriority:     src.AttackTargetPriority,
		HelixScl:           shot.HelixScl,
		HelixMag:           shot.HelixMag,
		HelixOffset:        shot.HelixOffset,
	}
	w.bulletNextID++
	w.bullets = append(w.bullets, b)
	w.emitAttackFireEffectsLocked(src)
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind: EntityEventBulletFired,
		Bullet: BulletEvent{
			Team:      b.Team,
			X:         b.X,
			Y:         b.Y,
			Angle:     angle,
			Damage:    b.Damage,
			BulletTyp: b.BulletType,
		},
	})
}

func (w *World) stepBullets(dt float32, idToIndex map[int32]int, spatial *entitySpatialIndex, teamSpatial map[TeamID]*entitySpatialIndex) {
	if len(w.bullets) == 0 {
		return
	}
	for i := 0; i < len(w.bullets); {
		b := &w.bullets[i]
		if isPersistentBeamBulletClass(b.BulletClass) {
			impacted, expired := w.stepPersistentBeamBullet(b, dt)
			impactRot := beamImpactRotation(*b)
			if impacted {
				tx, ty := beamEndPosition(*b)
				w.emitEffectLocked(b.HitEffect, tx, ty, impactRot)
			} else if expired {
				tx, ty := beamEndPosition(*b)
				w.emitEffectLocked(b.DespawnEffect, tx, ty, impactRot)
			}
			if !expired {
				i++
				continue
			}
			last := len(w.bullets) - 1
			w.bullets[i] = w.bullets[last]
			w.bullets = w.bullets[:last]
			continue
		}
		b.AgeSec += dt
		b.X += b.VX * dt
		b.Y += b.VY * dt
		if b.HelixScl > 0 && b.HelixMag != 0 {
			rot := float32(math.Atan2(float64(b.VY), float64(b.VX)) * 180 / math.Pi)
			side := float32(math.Sin(float64((b.AgeSec*60+b.HelixOffset)/b.HelixScl))) * b.HelixMag * dt * 60
			b.X += trnsx(rot, 0, side)
			b.Y += trnsy(rot, 0, side)
		}
		if handled, remove := w.absorbBulletByUnitAbilitiesLocked(b, dt); handled {
			if remove {
				last := len(w.bullets) - 1
				w.bullets[i] = w.bullets[last]
				w.bullets = w.bullets[:last]
				continue
			}
			i++
			continue
		}
		hit := false
		impacted := false
		if b.HitUnits {
			if idx, ok := findHitEnemyEntityIndex(*b, w.model.Entities, spatial, teamSpatial, b.Radius, b.TargetAir, b.TargetGround); ok && idx >= 0 && idx < len(w.model.Entities) {
				target := &w.model.Entities[idx]
				if remaining, absorbed := w.absorbEntityAbilityDamage(target, b.X, b.Y, b.Damage); absorbed {
					hit = true
					impacted = true
				} else {
					initialHealth := w.applyDamageToEntityProfile(target, remaining, bulletDamageApplyProfile(*b))
					applyPierceDamageLoss(&b.Damage, b.PierceDamageFactor, initialHealth)
					applySlow(target, b.SlowSec, b.SlowMul)
					w.applyStatusToEntity(target, b.StatusID, b.StatusName, b.StatusDuration)
					hit = true
					impacted = true
				}
				w.applyChainDamage(*b, idx)
				w.applySplashDamage(*b)
				if b.PierceRemain > 0 {
					b.PierceRemain--
					hit = false
				}
			}
		}
		if !hit && b.HitBuilds {
			if pos, _, _, ok := w.findNearestEnemyBuilding(RawEntity{X: b.X, Y: b.Y, Team: b.Team}, b.Radius); ok {
				initialHealth := float32(0)
				if pos >= 0 && int(pos) < len(w.model.Tiles) && w.model.Tiles[pos].Build != nil {
					initialHealth = w.model.Tiles[pos].Build.Health
				}
				if w.applyDamageToBuildingProfile(pos, b.Damage*b.BuildingDamage, bulletDamageApplyProfile(*b)) {
					applyPierceDamageLoss(&b.Damage, b.PierceDamageFactor, initialHealth)
					w.applySplashDamage(*b)
					hit = true
					impacted = true
					if b.PierceBuilding && b.PierceRemain > 0 {
						b.PierceRemain--
						hit = false
					}
				}
			}
		}
		expired := b.AgeSec >= b.LifeSec
		impactRot := float32(math.Atan2(float64(b.VY), float64(b.VX)) * 180 / math.Pi)
		if impacted {
			w.emitEffectLocked(b.HitEffect, b.X, b.Y, impactRot)
		} else if expired {
			w.emitEffectLocked(b.DespawnEffect, b.X, b.Y, impactRot)
		}
		if !hit && !expired {
			i++
			continue
		}
		if (hit || expired) && b.FragmentCount > 0 {
			w.spawnBulletFragments(*b)
		}
		last := len(w.bullets) - 1
		w.bullets[i] = w.bullets[last]
		w.bullets = w.bullets[:last]
	}
}

func (w *World) spawnBulletFragments(parent simBullet) {
	n := parent.FragmentCount
	if n <= 0 {
		return
	}
	baseAngle := float32(math.Atan2(float64(parent.VY), float64(parent.VX)) * 180 / math.Pi)
	spread := parent.FragmentSpread
	if spread <= 0 {
		spread = 20
	}
	for i := int32(0); i < n; i++ {
		t := float32(i)
		offset := float32(0)
		if n > 1 {
			offset = (t/float32(n-1))*spread - spread/2
		}
		randomSpread := parent.FragmentRand
		if randomSpread <= 0 {
			randomSpread = spread
		}
		ang := baseAngle + parent.FragmentAngle + float32(offset) + (rand.Float32()-0.5)*randomSpread
		rad := float32(ang * math.Pi / 180)
		template := parent.FragmentBullet
		speed := parent.FragmentSpeed
		life := parent.FragmentLife
		damage := parent.Damage * 0.45
		splashDamage := parent.SplashDamage * 0.45
		splashRadius := parent.SplashRadius * 0.5
		radius := float32(4)
		buildingDamage := parent.BuildingDamage
		armorMultiplier := parent.ArmorMultiplier
		maxDamageFraction := parent.MaxDamageFraction
		shieldDamageMul := parent.ShieldDamageMul
		pierceDamageFactor := parent.PierceDamageFactor
		pierceArmor := parent.PierceArmor
		bulletType := parent.BulletType
		bulletClass := parent.BulletClass
		pierce := int32(0)
		pierceBuilding := false
		statusID := parent.StatusID
		statusName := parent.StatusName
		statusDuration := parent.StatusDuration
		hitBuilds := parent.HitBuilds
		targetAir := parent.TargetAir
		targetGround := parent.TargetGround
		hitEffect := parent.HitEffect
		despawnEffect := parent.DespawnEffect
		fragCount := int32(0)
		fragSpread2 := float32(0)
		fragRand := float32(0)
		fragAngle := float32(0)
		fragVelMin := float32(0)
		fragVelMax := float32(0)
		fragLifeMin := float32(0)
		fragLifeMax := float32(0)
		var fragBullet *bulletRuntimeProfile
		if template != nil {
			if template.Speed > 0 {
				speed = template.Speed
			}
			if template.Lifetime > 0 {
				life = template.Lifetime
			}
			damage = template.Damage
			splashDamage = template.SplashDamage
			splashRadius = template.SplashRadius
			radius = maxf(template.HitSize*0.5, 4)
			buildingDamage = template.BuildingDamage
			armorMultiplier = template.ArmorMultiplier
			maxDamageFraction = template.MaxDamageFraction
			shieldDamageMul = template.ShieldDamageMul
			pierceDamageFactor = template.PierceDamageFactor
			pierceArmor = template.PierceArmor
			bulletType = template.BulletType
			bulletClass = template.ClassName
			pierce = template.Pierce
			pierceBuilding = template.PierceBuilding
			statusID = template.StatusID
			statusName = template.StatusName
			statusDuration = template.StatusDuration
			hitBuilds = template.HitBuildings
			targetAir = template.TargetAir
			targetGround = template.TargetGround
			hitEffect = template.HitEffect
			despawnEffect = template.DespawnEffect
			fragCount = template.FragmentCount
			fragSpread2 = template.FragmentSpread
			fragRand = template.FragmentRandom
			fragAngle = template.FragmentAngle
			fragVelMin = template.FragmentVelocityMin
			fragVelMax = template.FragmentVelocityMax
			fragLifeMin = template.FragmentLifeMin
			fragLifeMax = template.FragmentLifeMax
			fragBullet = cloneBulletRuntimeProfile(template.FragmentBullet)
		}
		speedMul := randomRange(parent.FragmentVelMin, parent.FragmentVelMax)
		if speedMul == 0 {
			speedMul = 1
		}
		lifeMul := randomRange(parent.FragmentLifeMin, parent.FragmentLifeMax)
		if lifeMul == 0 {
			lifeMul = 1
		}
		b := simBullet{
			ID:                 w.bulletNextID,
			Team:               parent.Team,
			X:                  parent.X,
			Y:                  parent.Y,
			VX:                 float32(math.Cos(float64(rad))) * speed * speedMul,
			VY:                 float32(math.Sin(float64(rad))) * speed * speedMul,
			Damage:             damage,
			SplashDamage:       splashDamage,
			LifeSec:            maxf(life*lifeMul, 0.2),
			Radius:             radius,
			HitUnits:           parent.HitUnits,
			HitBuilds:          hitBuilds,
			BulletType:         bulletType,
			BulletClass:        bulletClass,
			SplashRadius:       splashRadius,
			BuildingDamage:     buildingDamage,
			ArmorMultiplier:    armorMultiplier,
			MaxDamageFraction:  maxDamageFraction,
			ShieldDamageMul:    shieldDamageMul,
			PierceDamageFactor: pierceDamageFactor,
			PierceArmor:        pierceArmor,
			SlowSec:            parent.SlowSec,
			SlowMul:            parent.SlowMul,
			PierceRemain:       pierce,
			PierceBuilding:     pierceBuilding,
			ChainCount:         0,
			ChainRange:         0,
			FragmentCount:      fragCount,
			FragmentSpread:     fragSpread2,
			FragmentRand:       fragRand,
			FragmentAngle:      fragAngle,
			FragmentVelMin:     fragVelMin,
			FragmentVelMax:     fragVelMax,
			FragmentLifeMin:    fragLifeMin,
			FragmentLifeMax:    fragLifeMax,
			FragmentBullet:     fragBullet,
			StatusID:           statusID,
			StatusName:         statusName,
			StatusDuration:     statusDuration,
			ShootEffect:        "",
			SmokeEffect:        "",
			HitEffect:          hitEffect,
			DespawnEffect:      despawnEffect,
			TargetAir:          targetAir,
			TargetGround:       targetGround,
			TargetPriority:     parent.TargetPriority,
		}
		w.bulletNextID++
		w.bullets = append(w.bullets, b)
		w.entityEvents = append(w.entityEvents, EntityEvent{
			Kind: EntityEventBulletFired,
			Bullet: BulletEvent{
				Team:      b.Team,
				X:         b.X,
				Y:         b.Y,
				Angle:     ang,
				Damage:    b.Damage,
				BulletTyp: b.BulletType,
			},
		})
	}
}

func (w *World) applySplashDamage(b simBullet) {
	if b.SplashRadius <= 0 || (b.SplashDamage <= 0 && b.StatusID == 0 && strings.TrimSpace(b.StatusName) == "") {
		return
	}
	// Damage enemy units in splash radius.
	for i := range w.model.Entities {
		e := &w.model.Entities[i]
		if e.Health <= 0 || e.Team == b.Team {
			continue
		}
		dx := e.X - b.X
		dy := e.Y - b.Y
		d2 := dx*dx + dy*dy
		if d2 > b.SplashRadius*b.SplashRadius {
			continue
		}
		dist := float32(math.Sqrt(float64(d2)))
		scale := 1 - 0.6*(dist/b.SplashRadius)
		if scale < 0.4 {
			scale = 0.4
		}
		if b.SplashDamage > 0 {
			if remaining, absorbed := w.absorbEntityAbilityDamage(e, b.X, b.Y, b.SplashDamage*scale); !absorbed {
				w.applyDamageToEntityProfile(e, remaining, bulletDamageApplyProfile(b))
			}
		}
		applySlow(e, b.SlowSec*scale, b.SlowMul)
		w.applyStatusToEntity(e, b.StatusID, b.StatusName, b.StatusDuration)
	}
	// Damage enemy buildings in splash radius.
	w.forEachEnemyBuildingInRange(b.Team, b.X, b.Y, b.SplashRadius, func(pos int32) {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			return
		}
		t := &w.model.Tiles[pos]
		if t.Build == nil || t.Build.Health <= 0 {
			return
		}
		px := float32(t.X*8 + 4)
		py := float32(t.Y*8 + 4)
		dx := px - b.X
		dy := py - b.Y
		d2 := dx*dx + dy*dy
		if d2 > b.SplashRadius*b.SplashRadius {
			return
		}
		dist := float32(math.Sqrt(float64(d2)))
		scale := 1 - 0.6*(dist/b.SplashRadius)
		if scale < 0.4 {
			scale = 0.4
		}
		if b.SplashDamage > 0 {
			_ = w.applyDamageToBuildingDetailed(pos, b.SplashDamage*scale*b.BuildingDamage)
		}
	})
}

func (w *World) applyChainDamage(b simBullet, firstIdx int) {
	if b.ChainCount <= 0 || b.ChainRange <= 0 || firstIdx < 0 || firstIdx >= len(w.model.Entities) {
		return
	}
	hit := map[int]struct{}{firstIdx: {}}
	prev := firstIdx
	for c := int32(0); c < b.ChainCount; c++ {
		next := -1
		bestDist2 := b.ChainRange * b.ChainRange
		px := w.model.Entities[prev].X
		py := w.model.Entities[prev].Y
		for i := range w.model.Entities {
			if _, exists := hit[i]; exists {
				continue
			}
			e := &w.model.Entities[i]
			if e.Health <= 0 || e.Team == b.Team {
				continue
			}
			dx := e.X - px
			dy := e.Y - py
			d2 := dx*dx + dy*dy
			if d2 > bestDist2 {
				continue
			}
			bestDist2 = d2
			next = i
		}
		if next < 0 {
			return
		}
		scale := float32(math.Pow(0.72, float64(c+1)))
		damage := b.Damage * scale
		target := &w.model.Entities[next]
		if remaining, absorbed := w.absorbEntityAbilityDamage(target, px, py, damage); !absorbed {
			w.applyDamageToEntityProfile(target, remaining, bulletDamageApplyProfile(b))
			applySlow(target, b.SlowSec*scale, b.SlowMul)
			w.applyStatusToEntity(target, b.StatusID, b.StatusName, b.StatusDuration)
		}
		hit[next] = struct{}{}
		prev = next
	}
}

func (w *World) applyBeamChainFromSource(src RawEntity, firstIdx int, sourceIsBuilding bool) {
	if src.AttackChainCount <= 0 || src.AttackChainRange <= 0 || firstIdx < 0 || firstIdx >= len(w.model.Entities) {
		return
	}
	hit := map[int]struct{}{firstIdx: {}}
	prev := firstIdx
	for c := int32(0); c < src.AttackChainCount; c++ {
		next := -1
		bestDist2 := src.AttackChainRange * src.AttackChainRange
		px := w.model.Entities[prev].X
		py := w.model.Entities[prev].Y
		for i := range w.model.Entities {
			if _, exists := hit[i]; exists {
				continue
			}
			e := &w.model.Entities[i]
			if e.Health <= 0 || e.Team == src.Team {
				continue
			}
			dx := e.X - px
			dy := e.Y - py
			d2 := dx*dx + dy*dy
			if d2 > bestDist2 {
				continue
			}
			bestDist2 = d2
			next = i
		}
		if next < 0 {
			return
		}
		scale := float32(math.Pow(0.72, float64(c+1)))
		dmg := src.AttackDamage * scale * w.outgoingDamageScale(src, sourceIsBuilding)
		target := &w.model.Entities[next]
		if remaining, absorbed := w.absorbEntityAbilityDamage(target, px, py, dmg); !absorbed {
			w.applyDamageToEntityProfile(target, remaining, attackDamageApplyProfile(src))
			applySlow(target, src.AttackSlowSec*scale, src.AttackSlowMul)
			w.applyStatusToEntity(target, src.AttackStatusID, src.AttackStatusName, src.AttackStatusDuration)
		}
		hit[next] = struct{}{}
		prev = next
	}
}

func (w *World) applyDamageToEntity(e *RawEntity, dmg float32) {
	w.applyDamageToEntityDetailed(e, dmg, false)
}

func (w *World) getBuildingWeaponProfile(blockID int16) (buildingWeaponProfile, bool) {
	name := w.blockNameByID(blockID)
	if name == "" {
		return buildingWeaponProfile{}, false
	}
	return w.buildingWeaponProfileByNameLocked(name)
}

func (w *World) findNearestEnemyBuilding(src RawEntity, rangeLimit float32) (int32, float32, float32, bool) {
	if w.model == nil || src.Team == 0 {
		return 0, 0, 0, false
	}
	bestDist2 := rangeLimit * rangeLimit
	bestPos := int32(0)
	var bestX, bestY float32
	found := false
	visitPos := func(pos int32) {
		if pos < 0 || int(pos) >= len(w.model.Tiles) {
			return
		}
		t := &w.model.Tiles[pos]
		if t.Build == nil || t.Build.Health <= 0 {
			return
		}
		if t.Build.Team == src.Team {
			return
		}
		tx := float32(t.X*8 + 4)
		ty := float32(t.Y*8 + 4)
		dx := tx - src.X
		dy := ty - src.Y
		d2 := dx*dx + dy*dy
		if d2 > bestDist2 {
			return
		}
		bestDist2 = d2
		bestPos = pos
		bestX = tx
		bestY = ty
		found = true
	}
	w.forEachEnemyBuildingInRange(src.Team, src.X, src.Y, rangeLimit, visitPos)
	if !found {
		return 0, 0, 0, false
	}
	return bestPos, bestX, bestY, true
}

func (w *World) applyDamageToBuilding(pos int32, damage float32) bool {
	return w.applyDamageToBuildingDetailed(pos, damage)
}

func (w *World) applyDamageToBuildingRaw(pos int32, damage float32) bool {
	if w.model == nil || damage <= 0 {
		return false
	}
	x := int(pos) % w.model.Width
	y := int(pos) / w.model.Width
	if !w.model.InBounds(x, y) {
		return false
	}
	t := &w.model.Tiles[y*w.model.Width+x]
	if t.Build == nil {
		return false
	}
	prevBlock := int16(t.Block)
	prevBlockName := w.blockNameByID(prevBlock)
	t.Build.Health -= damage
	if t.Build.Health > 0 {
		w.entityEvents = append(w.entityEvents, EntityEvent{
			Kind:     EntityEventBuildHealth,
			BuildPos: packTilePos(x, y),
			BuildHP:  t.Build.Health,
		})
		return true
	}
	team := t.Team
	powerRelevant := w.isPowerRelevantBuildingLocked(t)
	w.queueBrokenBuildPlanLocked(pos, t)
	w.removeActiveTileIndexLocked(pos, t)
	w.setBuildingOccupancyLocked(pos, t, false)
	t.Build = nil
	t.Block = 0
	delete(w.buildStates, pos)
	w.clearBuildingRuntimeLocked(pos)
	if powerRelevant {
		w.invalidatePowerNetsLocked()
	}
	if affectsCoreStorageLinks(prevBlockName) {
		w.refreshCoreStorageLinksLocked()
	}
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind:       EntityEventBuildDestroyed,
		BuildPos:   packTilePos(x, y),
		BuildTeam:  team,
		BuildBlock: prevBlock,
	})
	return true
}

func (w *World) blockSyncSuppressedLocked(pos int32) bool {
	if pos < 0 {
		return true
	}
	if _, ok := w.pendingBreaks[pos]; ok {
		return true
	}
	return false
}

func (w *World) acquireTrackedEntityTarget(
	src RawEntity,
	ents []RawEntity,
	idToIndex map[int32]int,
	spatial *entitySpatialIndex,
	teamSpatial map[TeamID]*entitySpatialIndex,
	rangeLimit float32,
	allowAir, allowGround bool,
	priority string,
	state *targetTrackState,
	dt float32,
	retargetDelay float32,
) (int32, bool) {
	if state == nil {
		return findNearestEnemyEntity(src, ents, spatial, teamSpatial, rangeLimit, allowAir, allowGround, priority)
	}
	if state.RetargetCD > 0 {
		state.RetargetCD -= dt
		if state.RetargetCD < 0 {
			state.RetargetCD = 0
		}
	}
	if state.TargetID != 0 {
		if idx, ok := findEntityIndexByID(ents, idToIndex, state.TargetID); ok {
			if targetStillValid(src, ents[idx], rangeLimit, allowAir, allowGround) {
				return state.TargetID, true
			}
		}
		state.TargetID = 0
	}
	if state.RetargetCD > 0 {
		return 0, false
	}
	tid, ok := findNearestEnemyEntity(src, ents, spatial, teamSpatial, rangeLimit, allowAir, allowGround, priority)
	if !ok {
		state.RetargetCD = emptyTargetRetargetDelay(retargetDelay)
		return 0, false
	}
	state.TargetID = tid
	state.RetargetCD = maxf(retargetDelay, 0.1)
	return tid, true
}

func emptyTargetRetargetDelay(retargetDelay float32) float32 {
	if retargetDelay <= 0 {
		return 0.12
	}
	return clampf(retargetDelay*0.5, 0.08, 0.25)
}

func findEntityIndexByID(ents []RawEntity, idToIndex map[int32]int, id int32) (int, bool) {
	if id == 0 {
		return -1, false
	}
	if idx, ok := idToIndex[id]; ok && idx >= 0 && idx < len(ents) && ents[idx].ID == id {
		return idx, true
	}
	for i := range ents {
		if ents[i].ID == id {
			return i, true
		}
	}
	return -1, false
}

func isPlayerControlledEntity(e RawEntity) bool {
	return e.PlayerID != 0
}

func targetStillValid(src RawEntity, target RawEntity, rangeLimit float32, allowAir, allowGround bool) bool {
	if src.Team == 0 || target.Health <= 0 || target.Team == 0 || target.Team == src.Team {
		return false
	}
	if !canTargetEntity(target, allowAir, allowGround) {
		return false
	}
	dx := target.X - src.X
	dy := target.Y - src.Y
	return dx*dx+dy*dy <= rangeLimit*rangeLimit
}

func findNearestEnemyEntity(src RawEntity, ents []RawEntity, spatial *entitySpatialIndex, teamSpatial map[TeamID]*entitySpatialIndex, rangeLimit float32, allowAir, allowGround bool, priority string) (int32, bool) {
	if src.Team == 0 {
		return 0, false
	}
	if !allowAir && !allowGround {
		allowAir, allowGround = true, true
	}
	bestDist2 := rangeLimit * rangeLimit
	bestID := int32(0)
	bestScore := float32(math.MaxFloat32)
	bestPriority := float32(-99999)
	visit := func(i int) {
		if i < 0 || i >= len(ents) {
			return
		}
		e := ents[i]
		if e.ID == src.ID || e.Health <= 0 {
			return
		}
		if e.Team == 0 || e.Team == src.Team {
			return
		}
		if !canTargetEntity(e, allowAir, allowGround) {
			return
		}
		dx := e.X - src.X
		dy := e.Y - src.Y
		d2 := dx*dx + dy*dy
		if d2 > bestDist2 {
			return
		}
		score := targetPriorityScore(src, e, d2, priority)
		targetPriority := entityTargetPriorityValue(e)
		if bestID == 0 || targetPriority > bestPriority || (targetPriority >= bestPriority && score < bestScore) {
			bestPriority = targetPriority
			bestScore = score
			bestDist2 = d2
			bestID = e.ID
		}
	}
	if len(teamSpatial) != 0 {
		for team, idx := range teamSpatial {
			if team == 0 || team == src.Team || idx == nil {
				continue
			}
			idx.forEachInRange(src.X, src.Y, rangeLimit, visit)
		}
	} else if spatial != nil {
		spatial.forEachInRange(src.X, src.Y, rangeLimit, visit)
	} else {
		for i := range ents {
			visit(i)
		}
	}
	return bestID, bestID != 0
}

func buildEntitySpatialIndex(ents []RawEntity) *entitySpatialIndex {
	if len(ents) == 0 {
		return nil
	}
	const entitySpatialCellSize = 64
	idx := &entitySpatialIndex{
		cellSize: entitySpatialCellSize,
		cells:    make(map[int64][]int, len(ents)),
	}
	for i := range ents {
		cx := int(math.Floor(float64(ents[i].X) / float64(entitySpatialCellSize)))
		cy := int(math.Floor(float64(ents[i].Y) / float64(entitySpatialCellSize)))
		key := packSpatialCell(cx, cy)
		idx.cells[key] = append(idx.cells[key], i)
	}
	return idx
}

func buildTeamEntitySpatialIndexes(ents []RawEntity) map[TeamID]*entitySpatialIndex {
	if len(ents) == 0 {
		return nil
	}
	const entitySpatialCellSize = 64
	out := make(map[TeamID]*entitySpatialIndex)
	for i := range ents {
		team := ents[i].Team
		if team == 0 {
			continue
		}
		idx := out[team]
		if idx == nil {
			idx = &entitySpatialIndex{
				cellSize: entitySpatialCellSize,
				cells:    map[int64][]int{},
			}
			out[team] = idx
		}
		cx := int(math.Floor(float64(ents[i].X) / float64(entitySpatialCellSize)))
		cy := int(math.Floor(float64(ents[i].Y) / float64(entitySpatialCellSize)))
		key := packSpatialCell(cx, cy)
		idx.cells[key] = append(idx.cells[key], i)
	}
	return out
}

func (w *World) forEachEnemyBuildingInRange(team TeamID, x, y, radius float32, visit func(pos int32)) {
	if w == nil || w.model == nil || team == 0 || radius < 0 || visit == nil {
		return
	}
	if len(w.teamBuildingSpatial) != 0 {
		for otherTeam, idx := range w.teamBuildingSpatial {
			if otherTeam == 0 || otherTeam == team || idx == nil {
				continue
			}
			idx.forEachInRange(x, y, radius, visit)
		}
		return
	}
	if len(w.teamBuildingTiles) != 0 {
		for otherTeam, positions := range w.teamBuildingTiles {
			if otherTeam == 0 || otherTeam == team {
				continue
			}
			for _, pos := range positions {
				visit(pos)
			}
		}
		return
	}
	rangeTiles := int(math.Ceil(float64(radius/8))) + 1
	centerX := int(x / 8)
	centerY := int(y / 8)
	minX := max(0, centerX-rangeTiles)
	maxX := min(w.model.Width-1, centerX+rangeTiles)
	minY := max(0, centerY-rangeTiles)
	maxY := min(w.model.Height-1, centerY+rangeTiles)
	for ty := minY; ty <= maxY; ty++ {
		row := ty * w.model.Width
		for tx := minX; tx <= maxX; tx++ {
			pos := int32(row + tx)
			tile := &w.model.Tiles[pos]
			if tile.Build == nil || tile.Block == 0 || tile.Build.Team == 0 || tile.Build.Team == team {
				continue
			}
			visit(pos)
		}
	}
}

func (idx *buildingSpatialIndex) insert(tileX, tileY int, pos int32) {
	if idx == nil || idx.cellSize <= 0 {
		return
	}
	cx := tileX * 8 / idx.cellSize
	cy := tileY * 8 / idx.cellSize
	key := packSpatialCell(cx, cy)
	idx.cells[key] = append(idx.cells[key], pos)
}

func (idx *buildingSpatialIndex) remove(tileX, tileY int, pos int32) {
	if idx == nil || idx.cellSize <= 0 {
		return
	}
	cx := tileX * 8 / idx.cellSize
	cy := tileY * 8 / idx.cellSize
	key := packSpatialCell(cx, cy)
	if cell, ok := idx.cells[key]; ok {
		cell = removeIndexedPosAll(cell, pos)
		if len(cell) == 0 {
			delete(idx.cells, key)
			return
		}
		idx.cells[key] = cell
	}
}

func packSpatialCell(x, y int) int64 {
	return (int64(int32(x)) << 32) | int64(uint32(y))
}

func (idx *buildingSpatialIndex) forEachInRange(x, y, radius float32, visit func(pos int32)) {
	if idx == nil || idx.cellSize <= 0 || visit == nil {
		return
	}
	cell := float32(idx.cellSize)
	minCX := int(math.Floor(float64((x - radius) / cell)))
	maxCX := int(math.Floor(float64((x + radius) / cell)))
	minCY := int(math.Floor(float64((y - radius) / cell)))
	maxCY := int(math.Floor(float64((y + radius) / cell)))
	for cy := minCY; cy <= maxCY; cy++ {
		for cx := minCX; cx <= maxCX; cx++ {
			for _, pos := range idx.cells[packSpatialCell(cx, cy)] {
				visit(pos)
			}
		}
	}
}

func (idx *entitySpatialIndex) forEachInRange(x, y, radius float32, visit func(i int)) {
	if idx == nil || idx.cellSize <= 0 || visit == nil {
		return
	}
	cell := float32(idx.cellSize)
	minCX := int(math.Floor(float64((x - radius) / cell)))
	maxCX := int(math.Floor(float64((x + radius) / cell)))
	minCY := int(math.Floor(float64((y - radius) / cell)))
	maxCY := int(math.Floor(float64((y + radius) / cell)))
	for cy := minCY; cy <= maxCY; cy++ {
		for cx := minCX; cx <= maxCX; cx++ {
			for _, i := range idx.cells[packSpatialCell(cx, cy)] {
				visit(i)
			}
		}
	}
}

func findHitEnemyEntityIndex(b simBullet, ents []RawEntity, spatial *entitySpatialIndex, teamSpatial map[TeamID]*entitySpatialIndex, radius float32, allowAir, allowGround bool) (int, bool) {
	if !allowAir && !allowGround {
		allowAir, allowGround = true, true
	}
	bestDist2 := float32(math.MaxFloat32)
	bestIdx := -1
	visit := func(i int) {
		if i < 0 || i >= len(ents) {
			return
		}
		e := ents[i]
		if e.Health <= 0 || e.Team == b.Team {
			return
		}
		if !canTargetEntity(e, allowAir, allowGround) {
			return
		}
		dx := e.X - b.X
		dy := e.Y - b.Y
		d2 := dx*dx + dy*dy
		hitR := radius + maxf(e.HitRadius, 1.0)
		if d2 > hitR*hitR {
			return
		}
		if d2 >= bestDist2 {
			return
		}
		bestDist2 = d2
		bestIdx = i
	}
	if len(teamSpatial) != 0 {
		for team, idx := range teamSpatial {
			if team == 0 || team == b.Team || idx == nil {
				continue
			}
			idx.forEachInRange(b.X, b.Y, radius+16, visit)
		}
	} else if spatial != nil {
		spatial.forEachInRange(b.X, b.Y, radius+16, visit)
	} else {
		for i := range ents {
			visit(i)
		}
	}
	return bestIdx, bestIdx >= 0
}

func targetPriorityScore(src RawEntity, e RawEntity, d2 float32, priority string) float32 {
	_ = src
	base := d2 - e.HitRadius*e.HitRadius
	if base < 0 {
		base = 0
	}
	dist := float32(math.Sqrt(float64(d2)))
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "lowest_health", "lowhp":
		return e.Health + dist*0.25
	case "highest_health", "highhp", "tank":
		return -e.Health + dist*0.35
	case "threat", "dps":
		threat := e.AttackDamage*1.8 + e.MaxHealth*0.15
		return -threat + dist*0.30
	default:
		return base
	}
}

func entityTargetPriorityValue(e RawEntity) float32 {
	// Vanilla UnitType.targetPriority defaults to 0; missile-style units are below that.
	if e.LifeSec > 0 && e.AgeSec >= 0 {
		return -1
	}
	return 0
}

func canTargetEntity(e RawEntity, allowAir, allowGround bool) bool {
	flying := isEntityFlying(e)
	if flying {
		return allowAir
	}
	return allowGround
}

func isEntityFlying(e RawEntity) bool {
	return e.Flying
}

func entityHitRadiusForType(typeID int16) float32 {
	if r, ok := entityHitRadiusByType[typeID]; ok && r > 0 {
		return r
	}
	return 4.8
}

func (w *World) ensureEntityDefaults(e *RawEntity) {
	if e == nil || e.RuntimeInit {
		return
	}
	if e.Health <= 0 {
		e.Health = 100
	}
	if e.MaxHealth <= 0 {
		e.MaxHealth = 100
	}
	if e.AttackRange <= 0 {
		e.AttackRange = 56
	}
	if e.AttackDamage <= 0 {
		e.AttackDamage = 8
	}
	if e.AttackInterval <= 0 {
		e.AttackInterval = 0.7
	}
	if e.AttackBulletSpeed <= 0 {
		e.AttackBulletSpeed = 34
	}
	if e.AttackBulletHitSize <= 0 {
		e.AttackBulletHitSize = 10
	}
	if !e.AttackBuildingDamageSet && e.AttackBuildingDamage != 0 {
		e.AttackBuildingDamageSet = true
	}
	if e.AttackSlowMul <= 0 {
		e.AttackSlowMul = 1
	}
	if e.SlowMul <= 0 {
		e.SlowMul = 1
	}
	if !e.AttackTargetAir && !e.AttackTargetGround {
		e.AttackTargetAir = true
		e.AttackTargetGround = true
	}
	if strings.TrimSpace(e.AttackTargetPriority) == "" {
		e.AttackTargetPriority = "nearest"
	}
	if e.StatusDamageMul <= 0 {
		e.StatusDamageMul = 1
	}
	if e.StatusHealthMul <= 0 {
		e.StatusHealthMul = 1
	}
	if e.StatusSpeedMul <= 0 {
		e.StatusSpeedMul = 1
	}
	if e.StatusReloadMul <= 0 {
		e.StatusReloadMul = 1
	}
	if e.StatusBuildSpeedMul <= 0 {
		e.StatusBuildSpeedMul = 1
	}
	if e.StatusDragMul <= 0 {
		e.StatusDragMul = 1
	}
	if e.StatusArmorOverride < 0 {
		e.StatusArmorOverride = -1
	}
	if e.HitRadius <= 0 {
		e.HitRadius = entityHitRadiusForType(e.TypeID)
	}
	if strings.TrimSpace(e.AttackFireMode) == "" {
		e.AttackFireMode = "projectile"
	}
	if e.ShieldMax < 0 {
		e.ShieldMax = 0
	}
	if e.Shield < 0 {
		e.Shield = 0
	}
	if e.ShieldRegen < 0 {
		e.ShieldRegen = 0
	}
	if e.Armor < 0 {
		e.Armor = 0
	}
	if prof, ok := w.unitRuntimeProfileForEntityLocked(*e); ok {
		w.applyUnitRuntimeProfile(e, prof)
	}
	w.applyWeaponProfile(e)
	e.RuntimeInit = true
}

func (w *World) applyWeaponProfile(e *RawEntity) {
	if e == nil {
		return
	}
	p := defaultWeaponProfile
	if name, ok := w.unitNamesByID[e.TypeID]; ok && name != "" {
		if byName, exists := w.unitProfilesByName[name]; exists {
			p = byName
			applyWeaponProfileToEntity(e, p)
			if e.HitRadius <= 0 {
				e.HitRadius = entityHitRadiusForType(e.TypeID)
			}
			return
		}
	}
	src := w.unitProfilesByType
	if len(src) == 0 {
		src = weaponProfilesByType
	}
	if v, ok := src[e.TypeID]; ok {
		p = v
	}
	if p != defaultWeaponProfile || len(src) > 0 {
		applyWeaponProfileToEntity(e, p)
	} else {
		w.applyWeaponFromUnitTypeDef(e)
	}
	if e.HitRadius <= 0 {
		e.HitRadius = entityHitRadiusForType(e.TypeID)
	}
}

func (w *World) applyUnitTypeDef(e *RawEntity) {
	if e == nil {
		return
	}
	if prof, ok := w.unitRuntimeProfileForEntityLocked(*e); ok {
		w.applyUnitRuntimeProfile(e, prof)
	}
	def, ok := vanilla.UnitTypeDef{}, false
	if w.unitTypeDefsByID != nil {
		def, ok = w.unitTypeDefsByID[e.TypeID]
	}
	if name := strings.TrimSpace(w.unitNamesByID[e.TypeID]); name != "" {
		if fallback, fallbackOK := fallbackCoreUnitTypeDef(name); fallbackOK {
			if !ok || def.Health <= 0 {
				def.Health = fallback.Health
			}
			if !ok || def.Armor <= 0 {
				def.Armor = fallback.Armor
			}
			if !ok || def.HitSize <= 0 {
				def.HitSize = fallback.HitSize
			}
			if !ok || def.Speed <= 0 {
				def.Speed = fallback.Speed
			}
			if !ok || def.RotateSpeed <= 0 {
				def.RotateSpeed = fallback.RotateSpeed
			}
			ok = true
		}
	} else if fallbackName := fallbackUnitNameByTypeID(e.TypeID); fallbackName != "" {
		if fallback, fallbackOK := fallbackCoreUnitTypeDef(fallbackName); fallbackOK {
			if !ok || def.Health <= 0 {
				def.Health = fallback.Health
			}
			if !ok || def.Armor <= 0 {
				def.Armor = fallback.Armor
			}
			if !ok || def.HitSize <= 0 {
				def.HitSize = fallback.HitSize
			}
			if !ok || def.Speed <= 0 {
				def.Speed = fallback.Speed
			}
			if !ok || def.RotateSpeed <= 0 {
				def.RotateSpeed = fallback.RotateSpeed
			}
			ok = true
		}
	}
	if !ok {
		return
	}
	if def.Health > 0 {
		if e.RuntimeInit {
			e.Health = def.Health
			e.MaxHealth = def.Health
		} else {
			if e.MaxHealth <= 0 {
				e.MaxHealth = def.Health
			}
			if e.Health <= 0 {
				e.Health = minf(e.MaxHealth, def.Health)
			}
		}
	}
	if def.Armor > 0 {
		e.Armor = def.Armor
	}
	if def.HitSize > 0 {
		e.HitRadius = def.HitSize
	}
	if def.Speed > 0 {
		e.MoveSpeed = def.Speed
	}
}

func (w *World) applyWeaponFromUnitTypeDef(e *RawEntity) bool {
	if e == nil || w.unitTypeDefsByID == nil {
		return false
	}
	def, ok := w.unitTypeDefsByID[e.TypeID]
	if !ok {
		return false
	}
	if def.Weapon.Damage <= 0 || def.Weapon.Interval <= 0 {
		return false
	}
	e.AttackRange = def.Weapon.Range
	e.AttackFireMode = def.Weapon.FireMode
	e.AttackDamage = def.Weapon.Damage
	e.AttackInterval = def.Weapon.Interval
	e.AttackBulletSpeed = def.Weapon.BulletSpeed
	e.AttackBulletHitSize = 10
	e.AttackSplashRadius = def.Weapon.SplashRadius
	e.AttackBuildingDamage = 1
	e.AttackBuildingDamageSet = true
	e.AttackPierce = def.Weapon.Pierce
	e.AttackShootEffect = ""
	e.AttackSmokeEffect = ""
	e.AttackHitEffect = ""
	e.AttackDespawnEffect = ""
	e.AttackTargetAir = def.Weapon.TargetAir
	e.AttackTargetGround = def.Weapon.TargetGround
	e.AttackBuildings = def.Weapon.TargetGround
	if strings.TrimSpace(e.AttackTargetPriority) == "" {
		e.AttackTargetPriority = "nearest"
	}
	return true
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func clampf(v, minV, maxV float32) float32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func approachf(value, target, amount float32) float32 {
	if value < target {
		value += amount
		if value > target {
			return target
		}
		return value
	}
	value -= amount
	if value < target {
		return target
	}
	return value
}

func applySlow(e *RawEntity, sec, mul float32) {
	if e == nil || sec <= 0 {
		return
	}
	if mul <= 0 {
		mul = 1
	}
	e.SlowRemain = maxf(e.SlowRemain, sec)
	e.SlowMul = clampf(minf(e.SlowMul, mul), 0.2, 1)
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func lookAt(x, y, tx, ty float32) float32 {
	return float32(math.Atan2(float64(ty-y), float64(tx-x)) * 180 / math.Pi)
}

func normalizeEffectName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || name == "none" {
		return ""
	}
	return name
}

func (w *World) emitEffectLocked(name string, x, y, rotation float32) {
	name = normalizeEffectName(name)
	if w == nil || name == "" {
		return
	}
	w.entityEvents = append(w.entityEvents, EntityEvent{
		Kind:       EntityEventEffect,
		EffectName: name,
		EffectX:    x,
		EffectY:    y,
		EffectRot:  rotation,
	})
}

func (w *World) emitAttackFireEffectsLocked(src RawEntity) {
	w.emitEffectLocked(src.AttackShootEffect, src.X, src.Y, src.Rotation)
	w.emitEffectLocked(src.AttackSmokeEffect, src.X, src.Y, src.Rotation)
}

func (w *World) emitAttackHitEffectLocked(src RawEntity, x, y float32) {
	w.emitEffectLocked(src.AttackHitEffect, x, y, src.Rotation)
}

func (w *World) emitAttackDespawnEffectLocked(src RawEntity, x, y float32) {
	w.emitEffectLocked(src.AttackDespawnEffect, x, y, src.Rotation)
}

func applyBehaviorMotion(e *RawEntity, ents []RawEntity, idToIndex map[int32]int) {
	speed := e.MoveSpeed
	if speed <= 0 {
		speed = 18
	}
	speed *= entitySpeedMultiplier(*e)
	switch e.Behavior {
	case "move":
		if reachedTarget(e.X, e.Y, e.PatrolAX, e.PatrolAY, 1.25) {
			e.Behavior = ""
			e.VelX, e.VelY = 0, 0
			return
		}
		setVelocityToTarget(e, e.PatrolAX, e.PatrolAY, speed, 1.25)
	case "follow":
		if e.TargetID == 0 {
			e.VelX, e.VelY = 0, 0
			return
		}
		idx, ok := idToIndex[e.TargetID]
		if !ok || idx < 0 || idx >= len(ents) {
			e.VelX, e.VelY = 0, 0
			return
		}
		tx := ents[idx].X
		ty := ents[idx].Y
		setVelocityToTarget(e, tx, ty, speed, 1.25)
	case "patrol":
		tx, ty := e.PatrolAX, e.PatrolAY
		if e.PatrolToB {
			tx, ty = e.PatrolBX, e.PatrolBY
		}
		if reachedTarget(e.X, e.Y, tx, ty, 1.25) {
			e.PatrolToB = !e.PatrolToB
			tx, ty = e.PatrolAX, e.PatrolAY
			if e.PatrolToB {
				tx, ty = e.PatrolBX, e.PatrolBY
			}
		}
		setVelocityToTarget(e, tx, ty, speed, 1.25)
	}
}

func setVelocityToTarget(e *RawEntity, tx, ty, speed, stopRadius float32) {
	dx := tx - e.X
	dy := ty - e.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if dist <= stopRadius || dist == 0 {
		e.VelX, e.VelY = 0, 0
		return
	}
	e.VelX = speed * dx / dist
	e.VelY = speed * dy / dist
	e.Rotation = float32(math.Atan2(float64(dy), float64(dx)) * 180 / math.Pi)
}

func reachedTarget(x, y, tx, ty, radius float32) bool {
	dx := tx - x
	dy := ty - y
	return dx*dx+dy*dy <= radius*radius
}

// applyRulesToEntities 应用规则倍率到所有单位和建筑
func (w *World) applyRulesToEntities() {
	if w.model == nil {
		return
	}
	// 暂时不 aplicar倍率，因为 Unit/Building 结构与 Rules 方法不兼容
}

// GetRulesManager 返回规则管理器
func (w *World) GetRulesManager() *RulesManager {
	return w.rulesMgr
}

// GetWaveManager 返回波次管理器
func (w *World) GetWaveManager() *WaveManager {
	return w.wavesMgr
}

// triggerWave 触发波次生成
func (w *World) triggerWave(wm *WaveManager) {
	// Always advance wave counter when wave is triggered.
	nextWave := w.wave + 1
	w.wave = nextWave

	if w.model == nil {
		return
	}

	plan := wm.GeneratePlan(nextWave)
	if plan == nil {
		return
	}

	_, waveTeam := w.teamsFromRulesLocked()

	// 生成敌人（使用 RawEntity 结构）
	for group := 0; group < int(plan.GroupCount); group++ {
		for unitIdx := 0; unitIdx < int(plan.GroupSize); unitIdx++ {
			if len(w.model.Entities) >= 200 {
				break // 限制最大单位数量
			}

			enemyType := plan.EnemyTypePrior[0]
			if len(plan.EnemyTypePrior) > 0 {
				enemyType = plan.EnemyTypePrior[group%len(plan.EnemyTypePrior)]
			}

			posX, posY, ok := w.pickWaveSpawnPositionLocked(enemyType, waveTeam)
			if !ok {
				posX = float32(w.model.Width*8) / 2
				posY = float32(w.model.Height*8) / 2
			}
			posX += float32((unitIdx%3)-1) * 8
			posY += float32((group%3)-1) * 8
			posX = clampf(posX, 0, float32(w.model.Width*8))
			posY = clampf(posY, 0, float32(w.model.Height*8))
			w.addEnemy(enemyType, posX, posY)
		}

	}
}

// addEnemy 添加敌方单位
func (w *World) addEnemy(unitType int16, x, y float32) {
	if w.model == nil {
		return
	}

	unit := RawEntity{
		TypeID:       unitType,
		X:            x,
		Y:            y,
		Team:         2, // default fallback, rules may override below
		Health:       100,
		MaxHealth:    100,
		AttackDamage: 10,
		SlowMul:      1,
		Rotation:     0,
		RuntimeInit:  true,
		MineTilePos:  invalidEntityTilePos,
	}
	if _, waveTeam := w.teamsFromRulesLocked(); waveTeam != 0 {
		unit.Team = waveTeam
	}
	w.applyUnitTypeDef(&unit)
	w.applyWeaponProfile(&unit)
	if isEntityFlying(unit) {
		unit.Elevation = 1
	}
	w.model.AddEntity(unit)
}
