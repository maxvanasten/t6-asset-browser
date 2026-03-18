package t6assets

// AssetType represents the type of a game asset
type AssetType int

const (
	AssetTypeUnknown AssetType = iota
	AssetTypeXModel
	AssetTypeXAnim
	AssetTypeMaterial
	AssetTypeImage
	AssetTypeWeapon
	AssetTypeSound
	AssetTypeFX
	AssetTypePerk
)

func (at AssetType) String() string {
	switch at {
	case AssetTypeXModel:
		return "xmodel"
	case AssetTypeXAnim:
		return "xanim"
	case AssetTypeMaterial:
		return "material"
	case AssetTypeImage:
		return "image"
	case AssetTypeWeapon:
		return "weapon"
	case AssetTypeSound:
		return "sound"
	case AssetTypeFX:
		return "fx"
	case AssetTypePerk:
		return "perk"
	default:
		return "unknown"
	}
}

// Asset represents a game asset
type Asset struct {
	Name   string    // Asset name (from string table)
	Type   AssetType // Asset type
	Source string    // Source FastFile (e.g., "zm_tomb.ff")
}

// Weapon represents a weapon asset
type Weapon struct {
	Asset
	DisplayName    string
	ViewModel      string
	WorldModel     string
	Damage         int
	FireRate       float64
	MagazineSize   int
	MaxAmmo        int
	WeaponType     WeaponType
	IsWonderWeapon bool
	IsUpgraded     bool
	Materials      []string
	Sounds         []string
	Animations     []string
}

// WeaponType represents weapon classifications
type WeaponType int

const (
	WeaponPistol WeaponType = iota
	WeaponSMG
	WeaponRifle
	WeaponShotgun
	WeaponLMG
	WeaponSniper
	WeaponLauncher
	WeaponWonder
)

func (wt WeaponType) String() string {
	switch wt {
	case WeaponPistol:
		return "pistol"
	case WeaponSMG:
		return "smg"
	case WeaponRifle:
		return "rifle"
	case WeaponShotgun:
		return "shotgun"
	case WeaponLMG:
		return "lmg"
	case WeaponSniper:
		return "sniper"
	case WeaponLauncher:
		return "launcher"
	case WeaponWonder:
		return "wonder"
	default:
		return "unknown"
	}
}

// XModel represents a 3D model asset
type XModel struct {
	Asset
	Materials  []string
	BoneCount  int
	BoneNames  []string
	IsAnimated bool
	IsVehicle  bool
	IsPlayer   bool
}

// Perk represents a perk/specialty
type Perk struct {
	Asset
	DisplayName string
	Icon        string
	Cost        int
}

// Registry stores all discovered assets
type Registry struct {
	Assets   map[string]*Asset
	ByType   map[AssetType][]*Asset
	BySource map[string][]*Asset
}

// NewRegistry creates a new asset registry
func NewRegistry() *Registry {
	return &Registry{
		Assets:   make(map[string]*Asset),
		ByType:   make(map[AssetType][]*Asset),
		BySource: make(map[string][]*Asset),
	}
}

// Add adds an asset to the registry
func (r *Registry) Add(asset *Asset) {
	key := asset.Source + "/" + asset.Name
	r.Assets[key] = asset
	r.ByType[asset.Type] = append(r.ByType[asset.Type], asset)
	r.BySource[asset.Source] = append(r.BySource[asset.Source], asset)
}

// GetByType returns all assets of a given type
func (r *Registry) GetByType(t AssetType) []*Asset {
	return r.ByType[t]
}

// GetBySource returns all assets from a given source
func (r *Registry) GetBySource(source string) []*Asset {
	return r.BySource[source]
}
