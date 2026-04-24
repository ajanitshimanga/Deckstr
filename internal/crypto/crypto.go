package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id parameters (OWASP recommended)
	argonTime    = 3         // Number of iterations
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4         // Parallelism
	argonKeyLen  = 32        // 256 bits for AES-256

	// Salt and nonce sizes
	saltSize  = 16 // 128 bits
	nonceSize = 12 // 96 bits for GCM
)

var (
	ErrDecryptionFailed = errors.New("decryption failed: invalid password or corrupted data")
	ErrInvalidData      = errors.New("invalid encrypted data format")
)

// CryptoService handles encryption and key derivation
type CryptoService struct{}

// NewCryptoService creates a new crypto service instance
func NewCryptoService() *CryptoService {
	return &CryptoService{}
}

// GenerateSalt creates a random salt for key derivation
func (c *CryptoService) GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GenerateNonce creates a random nonce for AES-GCM
func (c *CryptoService) GenerateNonce() ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return nonce, nil
}

// DeriveKey derives an encryption key from a password using Argon2id
func (c *CryptoService) DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		argonKeyLen,
	)
}

// Encrypt encrypts plaintext using AES-256-GCM
func (c *CryptoService) Encrypt(plaintext []byte, key []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size: got %d, want %d", len(nonce), gcm.NonceSize())
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (c *CryptoService) Decrypt(ciphertext []byte, key []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size: got %d, want %d", len(nonce), gcm.NonceSize())
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptWithPassword is a convenience method that handles key derivation
func (c *CryptoService) EncryptWithPassword(plaintext []byte, password string) (salt, nonce, ciphertext []byte, err error) {
	salt, err = c.GenerateSalt()
	if err != nil {
		return nil, nil, nil, err
	}

	nonce, err = c.GenerateNonce()
	if err != nil {
		return nil, nil, nil, err
	}

	key := c.DeriveKey(password, salt)
	ciphertext, err = c.Encrypt(plaintext, key, nonce)
	if err != nil {
		return nil, nil, nil, err
	}

	return salt, nonce, ciphertext, nil
}

// DecryptWithPassword is a convenience method that handles key derivation
func (c *CryptoService) DecryptWithPassword(ciphertext []byte, password string, salt, nonce []byte) ([]byte, error) {
	key := c.DeriveKey(password, salt)
	return c.Decrypt(ciphertext, key, nonce)
}

// EncodeBase64 encodes bytes to base64 string
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 decodes base64 string to bytes
func DecodeBase64(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

// ClearBytes zeros out a byte slice (for clearing sensitive data from memory)
func ClearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// RecoveryPhraseWordCount is the number of words in a recovery phrase
const RecoveryPhraseWordCount = 6

// GenerateRecoveryPhrase generates a 6-word recovery phrase using cryptographically secure randomness
func (c *CryptoService) GenerateRecoveryPhrase() (string, error) {
	words := make([]string, RecoveryPhraseWordCount)
	wordListLen := big.NewInt(int64(len(wordList)))

	for i := 0; i < RecoveryPhraseWordCount; i++ {
		idx, err := rand.Int(rand.Reader, wordListLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random index: %w", err)
		}
		words[i] = wordList[idx.Int64()]
	}

	return strings.Join(words, " "), nil
}

// HKDF info labels for recovery key derivation (v2 scheme).
//
// These strings provide domain separation per RFC 5869 §3.2 so the same
// phrase + salt can produce independent keys for verification vs.
// encryption. Security does not depend on these strings being secret —
// only on them being distinct across contexts. The "deckstr.org/v1/"
// prefix makes collisions with any other HKDF user impossible even if
// the code were embedded in a larger application. The "v1" in the label
// is the label-scheme version (not the vault schema version) so we can
// evolve labels independently of the wire format.
const (
	recoveryVerifyInfo  = "deckstr.org/v1/recovery/verify"
	recoveryEncryptInfo = "deckstr.org/v1/recovery/encrypt"

	// recoveryKeyLen matches AES-256 so the same 32 bytes can be used
	// directly as an AES key for the encrypt derivation.
	recoveryKeyLen = 32
)

// normalizeRecoveryPhrase collapses the user's typed phrase into a canonical
// form so minor typing variations still verify: lowercase, trim leading and
// trailing whitespace, and collapse runs of internal whitespace (including
// tabs and newlines) into single spaces. Without the internal collapse,
// "apple  bear" (two spaces) would hash differently than "apple bear".
func normalizeRecoveryPhrase(phrase string) string {
	return strings.Join(strings.Fields(strings.ToLower(phrase)), " ")
}

// DeriveRecoveryKeys derives two independent 32-byte keys from the recovery
// phrase using Argon2id as the rate-limited base step followed by HKDF-SHA256
// expansion into distinct domain-separated outputs.
//
//   master     = Argon2id(normalize(phrase), salt)         // slow, rate-limited
//   verifyHash = HKDF-SHA256(master, nil, "…/recovery/verify",  32)
//   encryptKey = HKDF-SHA256(master, nil, "…/recovery/encrypt", 32)
//
// verifyHash is safe to store on disk (its only job is a constant-time
// equality check). encryptKey is used to wrap the master vault key and is
// reconstructible only from the phrase plus salt — never persisted.
//
// This replaces the v1 scheme (HashRecoveryPhrase used for BOTH outputs)
// where the stored verification hash was literally the encryption key,
// letting anyone with read access to vault.osm decrypt the vault without
// ever knowing the phrase. The v2 scheme closes that hole: HKDF guarantees
// that verifyHash reveals nothing about encryptKey and vice versa.
func (c *CryptoService) DeriveRecoveryKeys(phrase string, salt []byte) (verifyHash, encryptKey []byte, err error) {
	normalized := normalizeRecoveryPhrase(phrase)
	master := argon2.IDKey(
		[]byte(normalized),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		argonKeyLen,
	)
	defer ClearBytes(master)

	verifyHash, err = hkdf.Key(sha256.New, master, nil, recoveryVerifyInfo, recoveryKeyLen)
	if err != nil {
		return nil, nil, fmt.Errorf("hkdf verify: %w", err)
	}
	encryptKey, err = hkdf.Key(sha256.New, master, nil, recoveryEncryptInfo, recoveryKeyLen)
	if err != nil {
		return nil, nil, fmt.Errorf("hkdf encrypt: %w", err)
	}
	return verifyHash, encryptKey, nil
}

// VerifyRecoveryPhraseV2 checks a phrase against a stored v2 verification
// hash using constant-time comparison. Returns (false, nil) on mismatch,
// (true, nil) on match, and propagates derivation errors. Both derived
// halves (verify hash and encrypt key) are zeroed before return — verify
// only needs one but we don't want the unused encrypt key lingering on
// the heap until GC for consistency with other call sites.
func (c *CryptoService) VerifyRecoveryPhraseV2(phrase string, salt []byte, storedVerifyHash []byte) (bool, error) {
	verifyHash, encryptKey, err := c.DeriveRecoveryKeys(phrase, salt)
	if err != nil {
		return false, err
	}
	defer ClearBytes(verifyHash)
	defer ClearBytes(encryptKey)
	return subtle.ConstantTimeCompare(verifyHash, storedVerifyHash) == 1, nil
}

// HashRecoveryPhrase hashes a recovery phrase using Argon2id (legacy v1
// scheme). Deprecated: the v1 scheme used this same output as both the
// verification hash AND the key that encrypts the master vault key, which
// means an attacker with read access to vault.osm could decrypt the vault
// without ever knowing the phrase. Retained ONLY to read and migrate v1
// vaults in storage. New code must use DeriveRecoveryKeys.
func (c *CryptoService) HashRecoveryPhrase(phrase string, salt []byte) []byte {
	// Normalize: lowercase and trim whitespace
	normalized := strings.ToLower(strings.TrimSpace(phrase))
	return c.DeriveKey(normalized, salt)
}

// VerifyRecoveryPhrase checks a phrase against a v1 verification hash using
// constant-time comparison. Legacy — see HashRecoveryPhrase.
func (c *CryptoService) VerifyRecoveryPhrase(phrase string, salt []byte, storedHash []byte) bool {
	computedHash := c.HashRecoveryPhrase(phrase, salt)
	defer ClearBytes(computedHash)
	if len(computedHash) != len(storedHash) {
		return false
	}
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1
}

// wordList is a curated list of simple, unambiguous English words for recovery phrases.
// Using 256 words gives ~8 bits of entropy per word, so 6 words = ~48 bits.
// With rate limiting, this provides adequate security for a recovery mechanism.
var wordList = []string{
	"apple", "arrow", "badge", "beach", "berry", "blade", "bloom", "board",
	"boat", "bold", "bone", "book", "brave", "bread", "brick", "bridge",
	"bright", "bring", "brook", "brush", "cabin", "calm", "camp", "candy",
	"cape", "card", "carry", "castle", "cave", "chain", "chair", "chalk",
	"charm", "chase", "chest", "chief", "child", "chip", "city", "claim",
	"class", "clay", "clean", "clear", "cliff", "climb", "clock", "close",
	"cloth", "cloud", "coast", "cold", "color", "coral", "corn", "couch",
	"court", "cover", "craft", "crane", "cream", "creek", "crest", "crisp",
	"cross", "crown", "crush", "crystal", "curve", "dance", "dawn", "deer",
	"delta", "dense", "depth", "desk", "digit", "diver", "dock", "door",
	"draft", "dragon", "drain", "dream", "dress", "drift", "drill", "drink",
	"drive", "drum", "dust", "eagle", "earth", "east", "edge", "elder",
	"ember", "empty", "equal", "event", "extra", "fable", "face", "fair",
	"faith", "falcon", "fame", "fancy", "farm", "feast", "fence", "ferry",
	"field", "final", "fire", "fish", "flag", "flame", "flash", "fleet",
	"flight", "float", "flock", "flood", "floor", "flour", "flower", "fluid",
	"foam", "focus", "forge", "form", "fort", "forum", "fossil", "found",
	"frame", "fresh", "friend", "front", "frost", "fruit", "fuel", "future",
	"game", "garden", "gate", "gather", "gaze", "gear", "ghost", "giant",
	"gift", "girl", "glad", "glass", "globe", "glory", "glow", "gold",
	"golf", "grace", "grain", "grand", "grape", "grass", "grave", "green",
	"grid", "grip", "ground", "group", "grove", "guard", "guide", "gulf",
	"habit", "hair", "hammer", "hand", "harbor", "harvest", "haven", "hawk",
	"heart", "heavy", "hedge", "hero", "hill", "hollow", "home", "honey",
	"honor", "hood", "hope", "horse", "hotel", "house", "human", "hunt",
	"icon", "image", "inch", "index", "inner", "input", "iron", "island",
	"ivory", "jade", "jazz", "jewel", "joint", "journey", "judge", "juice",
	"jump", "jungle", "justice", "keen", "keep", "kernel", "king", "kite",
	"knight", "knot", "label", "lace", "ladder", "lake", "lamp", "land",
	"lane", "laser", "lawn", "layer", "leaf", "learn", "lemon", "lens",
	"level", "liberty", "light", "lilac", "lime", "limit", "line", "lion",
	"list", "live", "load", "loan", "local", "lock", "lodge", "logic",
	"loop", "lotus", "love", "loyal", "luck", "lunar", "lunch", "luxury",
	"magic", "mango", "maple", "marble", "march", "margin", "marine", "market",
	"mask", "master", "match", "maze", "meadow", "medal", "melody", "melon",
	"member", "memory", "mentor", "merge", "merit", "metal", "method", "middle",
	"milk", "mind", "mine", "mint", "mirror", "mist", "model", "modern",
	"moment", "money", "monk", "moon", "moral", "morning", "mosaic", "motor",
	"mount", "mouse", "mouth", "movie", "mud", "muscle", "museum", "music",
	"mystery", "myth", "nail", "name", "narrow", "nation", "nature", "navy",
	"near", "neck", "needle", "nest", "neutral", "night", "noble", "noise",
	"north", "note", "novel", "nurse", "oak", "oasis", "object", "ocean",
	"olive", "onion", "open", "opera", "option", "orange", "orbit", "orchid",
	"order", "organ", "orient", "origin", "outer", "output", "oval", "oven",
	"owner", "oxygen", "oyster", "pace", "pack", "paddle", "page", "paint",
	"pair", "palace", "palm", "panda", "panel", "panic", "paper", "parade",
	"parent", "park", "party", "patch", "path", "patrol", "pause", "peace",
	"peach", "peak", "pearl", "pencil", "people", "pepper", "perfect", "permit",
	"person", "photo", "piano", "picnic", "piece", "pilot", "pine", "pink",
	"pipe", "pirate", "pitch", "pizza", "place", "plain", "plane", "planet",
	"plant", "plate", "play", "plaza", "pledge", "plum", "plunge", "pocket",
	"poem", "poet", "point", "polar", "police", "polish", "pond", "pony",
	"pool", "popular", "port", "postal", "potato", "pottery", "pound", "power",
	"praise", "predict", "present", "press", "price", "pride", "primary", "prince",
	"print", "prison", "private", "prize", "problem", "process", "produce", "profit",
	"program", "project", "promise", "proof", "property", "protect", "proud", "provide",
	"public", "pulse", "pumpkin", "punch", "pupil", "purple", "purpose", "puzzle",
	"pyramid", "quality", "quantum", "quarter", "queen", "quest", "question", "quick",
	"quiet", "quilt", "quote", "rabbit", "race", "rack", "radar", "radio",
	"rail", "rain", "rainbow", "raise", "ramp", "ranch", "random", "range",
	"rapid", "rare", "rate", "ratio", "raven", "razor", "reach", "read",
	"ready", "real", "reason", "rebel", "recall", "recipe", "record", "recover",
	"reduce", "reef", "reflect", "reform", "refuge", "refuse", "region", "regret",
	"regular", "reject", "relate", "release", "relief", "rely", "remain", "remedy",
	"remind", "remote", "remove", "render", "renew", "rent", "repair", "repeat",
	"replace", "report", "rescue", "resist", "resort", "resource", "response", "result",
	"return", "reveal", "review", "reward", "rhythm", "ribbon", "rice", "rich",
	"ridge", "rifle", "right", "rigid", "ring", "riot", "ripple", "rise",
	"risk", "ritual", "rival", "river", "road", "roast", "robot", "robust",
	"rock", "rocket", "romance", "roof", "room", "root", "rope", "rose",
	"rotate", "rough", "round", "route", "royal", "rubber", "rude", "rug",
	"rule", "runway", "rural", "rustic", "sacred", "saddle", "safari", "safe",
	"sail", "salad", "salmon", "salon", "salt", "salute", "sample", "sand",
	"santa", "satire", "sauce", "save", "scale", "scan", "scene", "school",
	"science", "scout", "screen", "script", "scroll", "sea", "search", "season",
	"seat", "second", "secret", "section", "secure", "seed", "segment", "select",
	"seminar", "senior", "sense", "series", "service", "session", "settle", "setup",
	"seven", "shadow", "shaft", "shallow", "shape", "share", "shark", "sharp",
	"sheep", "sheet", "shelf", "shell", "shelter", "sheriff", "shield", "shift",
	"shine", "ship", "shirt", "shock", "shoe", "shop", "shore", "short",
	"shoulder", "shove", "shovel", "show", "shrimp", "shuffle", "shy", "sibling",
	"sick", "side", "siege", "sight", "sign", "signal", "silent", "silk",
	"silly", "silver", "similar", "simple", "since", "sing", "sink", "sister",
	"size", "skate", "sketch", "skill", "skin", "skirt", "skull", "slab",
	"slam", "sleep", "slice", "slide", "slim", "slogan", "slope", "slush",
	"small", "smart", "smile", "smoke", "smooth", "snack", "snake", "snap",
	"sniff", "snow", "soap", "soccer", "social", "sock", "soda", "soft",
	"solar", "soldier", "solid", "solution", "solve", "song", "soon", "sort",
	"soul", "sound", "soup", "source", "south", "space", "spare", "spark",
	"speak", "special", "speed", "spell", "spend", "sphere", "spice", "spider",
	"spike", "spin", "spirit", "split", "sponge", "sponsor", "spoon", "sport",
	"spot", "spray", "spread", "spring", "spy", "square", "squeeze", "squirrel",
	"stable", "stadium", "staff", "stage", "stairs", "stake", "stamp", "stand",
	"standard", "star", "start", "state", "station", "statue", "stay", "steak",
	"steal", "steam", "steel", "stem", "step", "stick", "still", "sting",
	"stock", "stomach", "stone", "stool", "stop", "store", "storm", "story",
	"stove", "strategy", "street", "strike", "string", "strip", "strong", "struggle",
	"student", "study", "stuff", "stumble", "style", "subject", "submit", "subtle",
	"subway", "success", "such", "sudden", "suffer", "sugar", "suggest", "suit",
	"summer", "summit", "sun", "sunny", "sunset", "super", "supply", "supreme",
	"sure", "surface", "surge", "surprise", "surround", "survey", "suspect", "sustain",
	"swallow", "swamp", "swan", "swap", "swarm", "sweet", "swift", "swim",
	"swing", "switch", "sword", "symbol", "symptom", "syrup", "system", "table",
	"tackle", "tail", "talent", "talk", "tank", "tape", "target", "task",
	"taste", "tattoo", "taxi", "teach", "team", "tell", "temple", "tenant",
	"tennis", "tent", "term", "test", "text", "thank", "theme", "then",
	"theory", "there", "thick", "thing", "think", "third", "thought", "three",
	"thrive", "throw", "thumb", "thunder", "ticket", "tide", "tiger", "tilt",
	"timber", "time", "tiny", "tip", "tired", "tissue", "title", "toast",
	"today", "toddler", "toe", "together", "toilet", "token", "tomato", "tomorrow",
	"tone", "tongue", "tonight", "tool", "tooth", "top", "topic", "torch",
	"tornado", "tortoise", "toss", "total", "touch", "tough", "tour", "tourist",
	"toward", "tower", "town", "toy", "track", "trade", "traffic", "tragic",
	"trail", "train", "transfer", "trap", "trash", "travel", "tray", "treat",
	"tree", "trend", "trial", "tribe", "trick", "trigger", "trim", "trip",
	"trophy", "trouble", "truck", "true", "truly", "trumpet", "trust", "truth",
	"tube", "tuition", "tumble", "tuna", "tunnel", "turkey", "turn", "turtle",
	"tutor", "twelve", "twenty", "twice", "twin", "twist", "type", "typical",
	"ugly", "umbrella", "unable", "uncle", "under", "undo", "unfair", "unfold",
	"unhappy", "uniform", "unique", "unit", "universe", "unknown", "unlock", "until",
	"unusual", "unveil", "update", "upgrade", "upon", "upper", "upset", "urban",
	"urge", "usage", "useful", "useless", "usual", "utility", "vacant", "vacuum",
	"vague", "valid", "valley", "valve", "van", "vanilla", "vapor", "various",
	"vast", "vault", "vector", "vehicle", "velvet", "vendor", "venture", "venue",
	"verb", "verify", "version", "very", "vessel", "veteran", "viable", "vibrant",
	"victory", "video", "view", "village", "vintage", "violin", "virtual", "virus",
	"visa", "visit", "visual", "vital", "vivid", "vocal", "voice", "volcano",
	"volume", "vote", "voyage", "wage", "wagon", "wait", "walk", "wall",
	"walnut", "wander", "want", "warfare", "warm", "warrior", "wash", "wasp",
	"waste", "watch", "water", "wave", "wax", "way", "wealth", "weapon",
	"wear", "weather", "web", "wedding", "weekend", "weird", "welcome", "west",
	"wet", "whale", "what", "wheat", "wheel", "when", "where", "whip",
	"whisper", "wide", "wife", "wild", "will", "win", "window", "wine",
	"wing", "winner", "winter", "wire", "wisdom", "wise", "wish", "witness",
	"wolf", "woman", "wonder", "wood", "wool", "word", "work", "world",
	"worry", "worth", "wrap", "wreck", "wrestle", "wrist", "write", "wrong",
	"yard", "year", "yellow", "yes", "yesterday", "yield", "yoga", "young",
	"youth", "zebra", "zero", "zone", "zoo",
}
