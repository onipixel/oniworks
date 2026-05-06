// Seeder for OniGram — populates the database with realistic users, posts,
// stories, follows, likes, comments, and direct messages using free images.
// Run: go run ./cmd/seed
package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	// side-effect: registers migrations
	_ "onigram/database/migrations"

	"github.com/onipixel/oniworks/framework/config"
	"github.com/onipixel/oniworks/framework/database"
	"github.com/onipixel/oniworks/framework/migrations"
	"golang.org/x/crypto/bcrypt"
)

// ─── Personas ────────────────────────────────────────────────────

type persona struct {
	username  string
	email     string
	bio       string
	website   string
	avatarURL string // pravatar.cc with a stable seed
}

var personas = []persona{
	{
		username:  "ryu_street",
		email:     "ryu@onigram.test",
		bio:       "🏙️ Street photographer. Tokyo, NYC, London. The city never sleeps and neither do I.",
		website:   "https://ryustreet.photo",
		avatarURL: "https://i.pravatar.cc/300?img=11",
	},
	{
		username:  "bella_bakes",
		email:     "bella@onigram.test",
		bio:       "🧁 Pastry chef & baking content creator. Making the world sweeter, one layer at a time.",
		website:   "https://bellabakes.com",
		avatarURL: "https://i.pravatar.cc/300?img=45",
	},
	{
		username:  "omar_lens",
		email:     "omar@onigram.test",
		bio:       "📷 Landscape & portrait photographer. Based in Morocco. Light is everything.",
		website:   "https://omarlens.art",
		avatarURL: "https://i.pravatar.cc/300?img=6",
	},
	{
		username:  "zoe_moves",
		email:     "zoe@onigram.test",
		bio:       "🩰 Dancer & choreographer. Movement is my language. NYC Dance Academy.",
		website:   "https://zoemoves.com",
		avatarURL: "https://i.pravatar.cc/300?img=48",
	},
	{
		username:  "alex_captures",
		email:     "alex@onigram.test",
		bio:       "📸 Photographer & visual storyteller. Chasing light, one frame at a time.",
		website:   "https://alexcaptures.co",
		avatarURL: "https://i.pravatar.cc/300?img=8",
	},
	{
		username:  "sarah_travels",
		email:     "sarah@onigram.test",
		bio:       "✈️ Full-time traveler. 47 countries & counting. Always planning the next trip.",
		website:   "https://sarahtravels.blog",
		avatarURL: "https://i.pravatar.cc/300?img=47",
	},
	{
		username:  "mike_eats",
		email:     "mike@onigram.test",
		bio:       "🍜 Food photographer & home chef. Every dish is a story.",
		website:   "https://mikeeats.com",
		avatarURL: "https://i.pravatar.cc/300?img=3",
	},
	{
		username:  "luna_art",
		email:     "luna@onigram.test",
		bio:       "🎨 Digital artist & illustrator. Turning imagination into pixels.",
		website:   "https://lunaart.studio",
		avatarURL: "https://i.pravatar.cc/300?img=49",
	},
	{
		username:  "jake_fitness",
		email:     "jake@onigram.test",
		bio:       "💪 Personal trainer & nutrition coach. No shortcuts, just results.",
		website:   "https://jakefitness.app",
		avatarURL: "https://i.pravatar.cc/300?img=12",
	},
	{
		username:  "emma_style",
		email:     "emma@onigram.test",
		bio:       "👗 Fashion & lifestyle blogger. Style is a way to say who you are without speaking.",
		website:   "https://emmastyle.fashion",
		avatarURL: "https://i.pravatar.cc/300?img=44",
	},
	{
		username:  "david_builds",
		email:     "david@onigram.test",
		bio:       "🔧 Maker, tinkerer, software engineer. Building things that matter.",
		website:   "https://davidbuilds.dev",
		avatarURL: "https://i.pravatar.cc/300?img=15",
	},
	{
		username:  "maya_nature",
		email:     "maya@onigram.test",
		bio:       "🌿 Nature photographer & environmental advocate. Earth is the art.",
		website:   "https://mayanature.earth",
		avatarURL: "https://i.pravatar.cc/300?img=36",
	},
}

// ─── Post content ────────────────────────────────────────────────

type postContent struct {
	imageURL string
	caption  string
	owner    string // username
}

var postContents = []postContent{
	// alex_captures
	{"https://picsum.photos/seed/alex1/800/1000", "Golden hour never disappoints. 🌅 #photography #goldenhour", "alex_captures"},
	{"https://picsum.photos/seed/alex2/900/900", "Urban geometry — the city is full of patterns if you look closely.", "alex_captures"},
	{"https://picsum.photos/seed/alex3/800/600", "Long exposure magic. 30 seconds of pure light trails. 📷", "alex_captures"},
	{"https://picsum.photos/seed/alex4/700/900", "Portrait session in natural light. No flash, just the window. 🖤", "alex_captures"},

	// sarah_travels
	{"https://picsum.photos/seed/sarah1/900/600", "Santorini sunrise. Woke up at 4am and it was absolutely worth it. ☀️🇬🇷", "sarah_travels"},
	{"https://picsum.photos/seed/sarah2/800/1000", "Kyoto in cherry blossom season. This view stopped me in my tracks. 🌸🇯🇵", "sarah_travels"},
	{"https://picsum.photos/seed/sarah3/900/700", "The markets of Marrakech are a sensory overload in the best way. 🧡🇲🇦", "sarah_travels"},
	{"https://picsum.photos/seed/sarah4/800/800", "Iceland's Seljalandsfoss waterfall. You can actually walk behind it! 🇮🇸💧", "sarah_travels"},

	// mike_eats
	{"https://picsum.photos/seed/mike1/800/800", "Homemade ramen from scratch. 18-hour bone broth. Yes, every minute counts. 🍜", "mike_eats"},
	{"https://picsum.photos/seed/mike2/900/700", "Sunday morning stack. Blueberry ricotta pancakes with maple butter. 🥞", "mike_eats"},
	{"https://picsum.photos/seed/mike3/800/600", "Fresh pasta, good wine, better company. That's all you need. 🍝🍷", "mike_eats"},
	{"https://picsum.photos/seed/mike4/700/800", "Sushi omakase last night. Chef's choice and it was perfection. 🍣", "mike_eats"},

	// luna_art
	{"https://picsum.photos/seed/luna1/800/1000", "New digital piece dropping this week. Sneak peek 👀 #digitalart #illustration", "luna_art"},
	{"https://picsum.photos/seed/luna2/900/900", "Color study #47. Playing with complementary palettes today. 🎨", "luna_art"},
	{"https://picsum.photos/seed/luna3/800/800", "Collaboration with @sarah_travels — turning her travel photos into art! ✨", "luna_art"},

	// jake_fitness
	{"https://picsum.photos/seed/jake1/800/1000", "5am club. The discipline you build before the world wakes up defines you. 💪", "jake_fitness"},
	{"https://picsum.photos/seed/jake2/900/700", "Meal prep Sunday 🥗 Fueling the week right. DM me for the macros.", "jake_fitness"},
	{"https://picsum.photos/seed/jake3/800/900", "New PR on deadlifts today. Consistency > intensity. Always. 🏋️", "jake_fitness"},

	// emma_style
	{"https://picsum.photos/seed/emma1/800/1100", "Thrift find of the century. Vintage Chanel for $8. You have to believe! 🤍", "emma_style"},
	{"https://picsum.photos/seed/emma2/900/900", "Sunday outfit. Sometimes simple IS the statement. #OOTD", "emma_style"},
	{"https://picsum.photos/seed/emma3/800/1000", "Milan Fashion Week recap — the best street style moments. 🇮🇹✨", "emma_style"},

	// david_builds
	{"https://picsum.photos/seed/david1/900/700", "Built a custom mechanical keyboard over the weekend. Cherry MX Blues. Loud and proud. ⌨️", "david_builds"},
	{"https://picsum.photos/seed/david2/800/600", "New desk setup is finally complete. Minimal, fast, focused. 🖥️", "david_builds"},
	{"https://picsum.photos/seed/david3/900/800", "First hardware project — a Raspberry Pi weather station. Logs to a SQLite DB! 🌦️", "david_builds"},

	// maya_nature
	{"https://picsum.photos/seed/maya1/1000/700", "Redwood National Park. Standing next to a 2,000-year-old tree puts things in perspective. 🌲 #nature #photography #travel", "maya_nature"},
	{"https://picsum.photos/seed/maya2/900/900", "Monarch butterfly migration. Witnessed 50,000 butterflies today. 🦋 #nature #wildlife", "maya_nature"},
	{"https://picsum.photos/seed/maya3/800/600", "Bioluminescent bay, Puerto Rico. No filters. This is real. ✨🌊 #nature #travel #photography", "maya_nature"},
	{"https://picsum.photos/seed/maya4/900/700", "Dawn at the Grand Canyon. Everything glows pink for exactly 4 minutes. 🌄 #nature #landscape", "maya_nature"},

	// ryu_street
	{"https://picsum.photos/seed/ryu1/800/1000", "3am Shinjuku. The city's heartbeat never stops. 🏙️ #streetphotography #tokyo #photography", "ryu_street"},
	{"https://picsum.photos/seed/ryu2/900/700", "Steam, rain, and neon. The perfect NYC trifecta. 📸 #streetphotography #nyc #nightphotography", "ryu_street"},
	{"https://picsum.photos/seed/ryu3/800/800", "Found this doorway in Lisbon. Sometimes you just have to look up. #streetphotography #travel #architecture", "ryu_street"},

	// bella_bakes
	{"https://picsum.photos/seed/bella1/800/900", "Lavender honey cake with champagne buttercream. This one took 3 attempts to get right. Worth it. 🧁 #baking #food #pastry", "bella_bakes"},
	{"https://picsum.photos/seed/bella2/800/800", "Croissant lamination day. 48 layers of pure butter. #baking #pastry #food", "bella_bakes"},
	{"https://picsum.photos/seed/bella3/900/700", "Sunday sourdough. The crust crunch you can almost hear through the screen. 🍞 #baking #sourdough #food", "bella_bakes"},

	// omar_lens
	{"https://picsum.photos/seed/omar1/900/700", "Blue hour in the Sahara. The silence out here is deafening. 🏜️ #landscape #photography #travel #nature", "omar_lens"},
	{"https://picsum.photos/seed/omar2/800/1000", "Portrait series — faces of the medina. Shot on film. #portrait #photography #art", "omar_lens"},
	{"https://picsum.photos/seed/omar3/1000/700", "Atlas Mountains at dawn. -8°C and absolutely worth it. 🏔️ #landscape #photography #nature", "omar_lens"},

	// zoe_moves
	{"https://picsum.photos/seed/zoe1/800/1000", "Rehearsal day 47. The piece is finally clicking into place. 🩰 #dance #art #choreography", "zoe_moves"},
	{"https://picsum.photos/seed/zoe2/900/900", "Rooftop warm-up before the show. NYC skyline as my backdrop. ✨ #dance #nyc #art", "zoe_moves"},

	// ─── Round 2: hashtag + mention rich posts ───────────────────
	// alex_captures
	{"https://picsum.photos/seed/alex5/900/1100", "Shot this at blue hour with @omar_lens last weekend — two lenses, one moment. 📷 #photography #bluehour #collaboration #portrait", "alex_captures"},
	{"https://picsum.photos/seed/alex6/800/800", "Film grain and coffee. My whole personality. ☕ #filmphotography #analog #photography #aesthetic", "alex_captures"},
	{"https://picsum.photos/seed/alex7/1000/700", "Reflections in puddles after the rain. The best light always comes after the storm. #photography #streetphotography #goldenhour #weather", "alex_captures"},

	// sarah_travels
	{"https://picsum.photos/seed/sarah5/900/700", "Lost in the medina with @omar_lens as my guide. Best day of this whole trip. 🧡 #travel #morocco #adventure #culture", "sarah_travels"},
	{"https://picsum.photos/seed/sarah6/800/1000", "Lake Como at dawn. Absolutely nobody around. Just me and the Alps. 🏔️ #travel #italy #landscape #nature #wanderlust", "sarah_travels"},
	{"https://picsum.photos/seed/sarah7/900/900", "Bali temple at golden hour. Every corner here is a composition. 🌅 #travel #bali #photography #spirituality #goldenhour", "sarah_travels"},
	{"https://picsum.photos/seed/sarah8/800/600", "Norwegian fjords. No filter needed — this is just Earth being extra. 🇳🇴 #travel #nature #fjords #landscape #scandinavia", "sarah_travels"},

	// mike_eats
	{"https://picsum.photos/seed/mike5/800/900", "Neapolitan pizza from scratch. 900°F wood fire, 90 seconds. Life-changing. 🍕 #food #pizza #italian #cooking #foodphotography", "mike_eats"},
	{"https://picsum.photos/seed/mike6/900/800", "Dim sum Sunday with the crew. @bella_bakes brought dessert 👌 #food #dimsum #brunch #foodie #friends", "mike_eats"},
	{"https://picsum.photos/seed/mike7/800/700", "Sourdough grilled cheese. It's basically a life event. 🧀 #food #sourdough #cooking #grillcheese #foodphotography", "mike_eats"},

	// luna_art
	{"https://picsum.photos/seed/luna4/900/1100", "Commission piece inspired by @maya_nature's Grand Canyon shots. Surreal to paint real places. 🎨 #art #digitalart #illustration #fanart #nature", "luna_art"},
	{"https://picsum.photos/seed/luna5/800/800", "Typography exploration. Letters are just art we've given meaning to. #art #typography #design #graphicdesign #illustration", "luna_art"},
	{"https://picsum.photos/seed/luna6/900/900", "Neon + watercolour hybrid. Still figuring out if this works. What do you think? 🌈 #art #watercolour #neon #experimental #illustration", "luna_art"},

	// jake_fitness
	{"https://picsum.photos/seed/jake4/800/1000", "Marathon training week 8. 45 miles in the bank. @zoe_moves keeps me motivated. 🏃 #running #marathon #fitness #training #endurance", "jake_fitness"},
	{"https://picsum.photos/seed/jake5/900/700", "Cold plunge at 5am. Not for everyone but it's for me. 🧊 #fitness #coldplunge #recovery #wellness #mindset", "jake_fitness"},
	{"https://picsum.photos/seed/jake6/800/900", "Calisthenics park session. You don't need a gym. #fitness #calisthenics #bodyweight #outdoors #streetworkout", "jake_fitness"},

	// emma_style
	{"https://picsum.photos/seed/emma4/800/1100", "Capsule wardrobe update. 12 pieces, infinite outfits. @zoe_moves modelled these perfectly. 🤍 #fashion #capsulewardrobe #minimalist #ootd #style", "emma_style"},
	{"https://picsum.photos/seed/emma5/900/900", "Vintage market haul. All under $20. Sustainable fashion is not boring. ♻️ #fashion #vintage #thrift #sustainable #ootd", "emma_style"},
	{"https://picsum.photos/seed/emma6/800/1000", "Tokyo street style. Every person here is a walking lookbook. 🇯🇵 #fashion #streetstyle #tokyo #japan #travel", "emma_style"},

	// david_builds
	{"https://picsum.photos/seed/david4/900/700", "Built a timelapse rig for @maya_nature's next dark sky shoot. Can't wait to see the results. ⚙️ #maker #diy #tech #photography #engineering", "david_builds"},
	{"https://picsum.photos/seed/david5/800/600", "Learning Rust. My brain hurts but in a good way. 🦀 #programming #rust #tech #coding #developer", "david_builds"},
	{"https://picsum.photos/seed/david6/900/800", "3D printed this enclosure for my home server. Fits perfectly. 🖨️ #maker #3dprinting #homelab #tech #diy", "david_builds"},

	// maya_nature
	{"https://picsum.photos/seed/maya5/1000/700", "Aurora borealis from the timelapse rig @david_builds made. Worth every freezing hour. 🌌 #nature #aurora #nightphotography #landscape #iceland", "maya_nature"},
	{"https://picsum.photos/seed/maya6/900/900", "Hummingbird at 1/8000s. Patience is the whole game. 🐦 #nature #wildlife #photography #birdphotography #macro", "maya_nature"},
	{"https://picsum.photos/seed/maya7/800/600", "Mushroom forest after the rain. This planet is wild. 🍄 #nature #macro #forest #photography #fungi", "maya_nature"},

	// ryu_street
	{"https://picsum.photos/seed/ryu4/800/1000", "This alley in Osaka appeared in three of my dreams before I actually found it. 🏮 #streetphotography #osaka #japan #travel #photography", "ryu_street"},
	{"https://picsum.photos/seed/ryu5/900/700", "Hong Kong. The density is the art. 🌆 #streetphotography #hongkong #architecture #cityscape #photography", "ryu_street"},
	{"https://picsum.photos/seed/ryu6/800/800", "Shot on Tri-X 400. The grain is not a bug, it's the whole point. 🎞️ #filmphotography #analog #streetphotography #blackandwhite #photography", "ryu_street"},

	// bella_bakes
	{"https://picsum.photos/seed/bella4/800/900", "@mike_eats tested this recipe for me and gave it a 10/10. High praise from him. 🎂 #baking #cake #food #pastry #collaboration", "bella_bakes"},
	{"https://picsum.photos/seed/bella5/900/800", "Matcha tiramisu. Two cultures, one dessert. 🍵 #baking #matcha #tiramisu #fusion #food #pastry", "bella_bakes"},
	{"https://picsum.photos/seed/bella6/800/700", "Pain au chocolat. The layers took me two days but look at that cross section. 🥐 #baking #pastry #french #food #croissant", "bella_bakes"},

	// omar_lens
	{"https://picsum.photos/seed/omar4/900/700", "Shot this while @alex_captures was setting up his tripod. Sometimes the best frame is the candid one. 📸 #photography #portrait #streetphotography #documentary #collaboration", "omar_lens"},
	{"https://picsum.photos/seed/omar5/800/1000", "Hasselblad + Sahara + no phone signal = bliss. 🏜️ #photography #landscape #desert #analog #travel", "omar_lens"},
	{"https://picsum.photos/seed/omar6/1000/700", "Erg Chebbi at midnight. The Milky Way was so bright I could read by it. 🌌 #photography #nightphotography #landscape #astro #nature", "omar_lens"},

	// zoe_moves
	{"https://picsum.photos/seed/zoe3/800/1000", "Opening night. After 47 rehearsals and a lot of @jake_fitness's early morning texts, we're here. 🩰 #dance #performance #art #ballet #theatre", "zoe_moves"},
	{"https://picsum.photos/seed/zoe4/900/900", "Aerial silk training. Every bruise is a lesson. 🎪 #dance #aerial #fitness #circus #art", "zoe_moves"},
	{"https://picsum.photos/seed/zoe5/800/800", "Improvisation session in the studio. No plan, just music and movement. 🎵 #dance #improvisation #art #movement #choreography", "zoe_moves"},
}

// ─── Story images ────────────────────────────────────────────────

var storyImages = map[string][]string{
	"alex_captures": {"https://picsum.photos/seed/alexs1/500/900", "https://picsum.photos/seed/alexs2/500/900"},
	"sarah_travels": {"https://picsum.photos/seed/sarahs1/500/900", "https://picsum.photos/seed/sarahs2/500/900"},
	"mike_eats":     {"https://picsum.photos/seed/mikes1/500/900"},
	"luna_art":      {"https://picsum.photos/seed/lunas1/500/900"},
	"jake_fitness":  {"https://picsum.photos/seed/jakes1/500/900"},
	"emma_style":    {"https://picsum.photos/seed/emmas1/500/900", "https://picsum.photos/seed/emmas2/500/900"},
	"maya_nature":   {"https://picsum.photos/seed/mayas1/500/900"},
	"ryu_street":    {"https://picsum.photos/seed/ryus1/500/900"},
	"bella_bakes":   {"https://picsum.photos/seed/bellas1/500/900"},
	"omar_lens":     {"https://picsum.photos/seed/omars1/500/900"},
	"zoe_moves":     {"https://picsum.photos/seed/zoes1/500/900"},
}

// ─── Comments ────────────────────────────────────────────────────

var commentBank = []string{
	"This is absolutely stunning 😍",
	"Wow, the colors here are incredible!",
	"I need to visit this place ASAP 🙌",
	"Your work never ceases to amaze me ✨",
	"How is this even possible?! 🤯",
	"Goals. Pure goals.",
	"Saved this for inspiration 🔖",
	"@alex_captures you've outdone yourself again",
	"This made my whole day better 💙",
	"The composition is just perfect",
	"Drop the location?? 📍",
	"Been following you for years and you keep getting better",
	"Legitimately cannot stop looking at this",
	"This deserves way more likes",
	"Ok but this is the one 👏",
	"Sending this to everyone I know",
	"The lighting 😭🙌",
	"Can I commission something like this?",
	"Art in its purest form",
	"This is what I come to OniGram for 💫",
}

// ─── DM conversations ────────────────────────────────────────────

type dmThread struct {
	user1    string
	user2    string
	messages []struct{ from, body string }
}

var dmThreads = []dmThread{
	{
		user1: "sarah_travels",
		user2: "luna_art",
		messages: []struct{ from, body string }{
			{"sarah_travels", "Hey Luna! I absolutely love your latest piece ✨"},
			{"luna_art", "Thank you so much Sarah!! Your travel shots are always such inspiration for me 🌍"},
			{"sarah_travels", "I was wondering — would you ever want to collaborate? Like I shoot, you paint?"},
			{"luna_art", "Oh wow, yes! I've literally been thinking about that. Let's do it!!"},
			{"sarah_travels", "Amazing! I'm in Tokyo next month, I'll send you some raw files 📸"},
			{"luna_art", "Perfect. Can't wait!! This is going to be so good 🎨"},
		},
	},
	{
		user1: "alex_captures",
		user2: "mike_eats",
		messages: []struct{ from, body string }{
			{"mike_eats", "Alex! I need a photographer for my cookbook shoot 📚"},
			{"alex_captures", "Mike!! Are you serious?? I'd love that"},
			{"mike_eats", "100% serious. I'm thinking natural light, rustic setting. Very film-y"},
			{"alex_captures", "That's literally my style lol. When are you thinking?"},
			{"mike_eats", "End of this month? I'll have 40 dishes ready"},
			{"alex_captures", "I'm blocking off that whole week. Send me the details 🙌"},
			{"mike_eats", "Will do! And obviously you get to eat everything after 🍜"},
			{"alex_captures", "Best deal I've ever agreed to 😂"},
		},
	},
	{
		user1: "jake_fitness",
		user2: "emma_style",
		messages: []struct{ from, body string }{
			{"emma_style", "Jake, do you do online coaching? Asking for a friend 😅"},
			{"jake_fitness", "Hey! Yeah I do — what are the goals?"},
			{"emma_style", "I want to feel stronger without losing the aesthetic lol. Very specific I know"},
			{"jake_fitness", "That's actually super achievable! I'd recommend a resistance + pilates hybrid approach"},
			{"emma_style", "Ohh that sounds perfect. How do I sign up?"},
			{"jake_fitness", "DM me your availability and I'll send over an intake form this week"},
		},
	},
	{
		user1: "david_builds",
		user2: "maya_nature",
		messages: []struct{ from, body string }{
			{"david_builds", "Maya! Your bioluminescent bay photo is on my desktop wallpaper 🌊"},
			{"maya_nature", "Haha that makes me so happy!! It was genuinely magical to witness"},
			{"david_builds", "I'm building an automated timelapse rig — think it could work for nature shots?"},
			{"maya_nature", "Oh 100%. For aurora and star trails it would be incredible"},
			{"david_builds", "Would you want to test it out? I'll bring the tech, you bring the locations"},
			{"maya_nature", "Deal. I know a perfect dark sky spot 2 hours from here ✨"},
		},
	},
}

// ─── Main ────────────────────────────────────────────────────────

func main() {
	_ = config.LoadEnv(".env")

	port, _ := strconv.Atoi(env("DB_PORT", "5432"))
	db, err := database.Open(database.Config{
		Driver:   database.DriverPostgres,
		Host:     env("DB_HOST", "127.0.0.1"),
		Port:     port,
		Name:     env("DB_NAME", "onigram"),
		User:     env("DB_USER", "postgres"),
		Password: env("DB_PASSWORD", "password"),
		SSLMode:  "disable",
		MaxOpen:  10,
		MaxIdle:  3,
	})
	must(err, "connect db")
	database.SetDefault(db)

	// Run migrations first
	m := migrations.New(db.SQLDB(), string(db.Driver()))
	m.LoadRegistry()
	must(m.Migrate(context.Background()), "migrate")

	fmt.Println("🌱 Seeding OniGram...")

	// Ensure storage dirs exist
	for _, dir := range []string{"storage/avatars", "storage/posts", "storage/stories"} {
		os.MkdirAll(dir, 0755)
	}

	// ─── Users ───────────────────────────────────────────────────
	fmt.Println("👥 Creating users...")
	userIDs := map[string]int64{}

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	for _, p := range personas {
		// Check if already exists — scan into plain int64
		var existingID int64
		err := database.Raw(`SELECT id FROM users WHERE username = $1`, p.username).Scan(&existingID)
		if err == nil && existingID > 0 {
			userIDs[p.username] = existingID
			fmt.Printf("  ↩  @%s already exists (id=%d)\n", p.username, existingID)
			continue
		}

		// Download avatar
		rawPath, err := downloadImage(p.avatarURL, "storage/avatars", fmt.Sprintf("avatar_%s.jpg", p.username))
		avatarPath := ""
		if err != nil {
			fmt.Printf("  ⚠  avatar download failed for %s: %v\n", p.username, err)
		} else {
			avatarPath = "/" + strings.ReplaceAll(rawPath, "\\", "/")
		}

		var newID int64
		err = database.Raw(
			`INSERT INTO users (username, email, password_hash, bio, website, avatar_path, created_at, updated_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			p.username, p.email, string(hash), p.bio, p.website, avatarPath,
			time.Now().Add(-time.Duration(rand.Intn(90))*24*time.Hour), time.Now(),
		).Scan(&newID)
		must(err, "insert user "+p.username)
		userIDs[p.username] = newID
		fmt.Printf("  ✓  @%s (id=%d)\n", p.username, newID)
	}

	// ─── Follows ─────────────────────────────────────────────────
	fmt.Println("🤝 Creating follows...")
	followPairs := [][]string{
		{"alex_captures", "sarah_travels"}, {"alex_captures", "mike_eats"},
		{"alex_captures", "luna_art"}, {"alex_captures", "maya_nature"},
		{"sarah_travels", "alex_captures"}, {"sarah_travels", "luna_art"},
		{"sarah_travels", "emma_style"}, {"sarah_travels", "maya_nature"},
		{"mike_eats", "alex_captures"}, {"mike_eats", "jake_fitness"},
		{"mike_eats", "emma_style"}, {"mike_eats", "sarah_travels"},
		{"luna_art", "alex_captures"}, {"luna_art", "sarah_travels"},
		{"luna_art", "emma_style"}, {"luna_art", "maya_nature"},
		{"jake_fitness", "mike_eats"}, {"jake_fitness", "david_builds"},
		{"jake_fitness", "emma_style"},
		{"emma_style", "sarah_travels"}, {"emma_style", "luna_art"},
		{"emma_style", "alex_captures"}, {"emma_style", "jake_fitness"},
		{"david_builds", "jake_fitness"}, {"david_builds", "alex_captures"},
		{"david_builds", "maya_nature"},
		{"maya_nature", "alex_captures"}, {"maya_nature", "sarah_travels"},
		{"maya_nature", "luna_art"}, {"maya_nature", "david_builds"},
	}
	for _, pair := range followPairs {
		followerID := userIDs[pair[0]]
		followingID := userIDs[pair[1]]
		if followerID == 0 || followingID == 0 {
			continue
		}
		_ = database.Raw(
			`INSERT INTO follows (follower_id, following_id, created_at) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			followerID, followingID, time.Now().Add(-time.Duration(rand.Intn(60))*24*time.Hour),
		).Exec()
	}
	fmt.Printf("  ✓  %d follow relationships\n", len(followPairs))

	// ─── Posts ───────────────────────────────────────────────────
	fmt.Println("📸 Creating posts...")
	postIDs := map[string]int64{} // caption → id

	for _, pc := range postContents {
		ownerID := userIDs[pc.owner]
		if ownerID == 0 {
			continue
		}

		filename := fmt.Sprintf("post_%d_%d.jpg", ownerID, time.Now().UnixNano())
		imgPath, err := downloadImage(pc.imageURL, "storage/posts", filename)
		if err != nil {
			fmt.Printf("  ⚠  image download failed: %v\n", err)
			continue
		}
		urlPath := "/" + strings.ReplaceAll(imgPath, "\\", "/")

		createdAt := time.Now().Add(-time.Duration(rand.Intn(30))*24*time.Hour - time.Duration(rand.Intn(23))*time.Hour)
		var newPostID int64
		err = database.Raw(
			`INSERT INTO posts (user_id, image_path, caption, created_at, updated_at)
			 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			ownerID, urlPath, pc.caption, createdAt, createdAt,
		).Scan(&newPostID)
		if err != nil {
			fmt.Printf("  ⚠  post insert failed: %v\n", err)
			continue
		}
		postIDs[pc.caption[:20]] = newPostID
		fmt.Printf("  ✓  post id=%d by @%s\n", newPostID, pc.owner)
		time.Sleep(5 * time.Millisecond) // avoid duplicate UnixNano
	}

	// ─── Hashtags ────────────────────────────────────────────────
	fmt.Println("🏷️  Extracting hashtags from captions...")
	seedHashtags(postContents, postIDs)

	// ─── Stories ─────────────────────────────────────────────────
	fmt.Println("📖 Creating stories...")
	for username, imgURLs := range storyImages {
		ownerID := userIDs[username]
		if ownerID == 0 {
			continue
		}
		for i, imgURL := range imgURLs {
			filename := fmt.Sprintf("story_%d_%d.jpg", ownerID, time.Now().UnixNano()+int64(i))
			imgPath, err := downloadImage(imgURL, "storage/stories", filename)
			if err != nil {
				fmt.Printf("  ⚠  story image download failed: %v\n", err)
				continue
			}
			urlPath := "/" + strings.ReplaceAll(imgPath, "\\", "/")
			_ = database.Raw(
				`INSERT INTO stories (user_id, image_path, expires_at, created_at) VALUES ($1,$2,$3,$4)`,
				ownerID, urlPath, time.Now().Add(20*time.Hour), time.Now().Add(-time.Duration(i+1)*time.Hour),
			).Exec()
			time.Sleep(5 * time.Millisecond)
		}
		fmt.Printf("  ✓  %d stories for @%s\n", len(imgURLs), username)
	}

	// ─── Likes ───────────────────────────────────────────────────
	fmt.Println("❤️  Creating likes...")
	// Get all post IDs
	type postRow struct {
		ID     int64 `db:"id"`
		UserID int64 `db:"user_id"`
	}
	var allPosts []postRow
	_ = database.Raw(`SELECT id, user_id FROM posts ORDER BY id`).All(&allPosts)

	likeCount := 0
	for _, post := range allPosts {
		// Random subset of users like each post (3–7 likes)
		likers := shuffle(keys(userIDs))
		numLikes := 3 + rand.Intn(5)
		if numLikes > len(likers) {
			numLikes = len(likers)
		}
		for _, liker := range likers[:numLikes] {
			likerID := userIDs[liker]
			if likerID == post.UserID {
				continue // don't like own posts
			}
			err := database.Raw(
				`INSERT INTO likes (user_id, post_id, created_at) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
				likerID, post.ID, time.Now().Add(-time.Duration(rand.Intn(720))*time.Minute),
			).Exec()
			if err == nil {
				likeCount++
			}
		}
	}
	fmt.Printf("  ✓  %d likes\n", likeCount)

	// ─── Comments ────────────────────────────────────────────────
	fmt.Println("💬 Creating comments...")
	commentCount := 0
	commenters := keys(userIDs)
	for _, post := range allPosts {
		numComments := 2 + rand.Intn(4)
		shuffled := shuffle(commenters)
		for i := 0; i < numComments && i < len(shuffled); i++ {
			commenter := shuffled[i]
			commenterID := userIDs[commenter]
			body := commentBank[rand.Intn(len(commentBank))]
			createdAt := time.Now().Add(-time.Duration(rand.Intn(360))*time.Minute)
			_ = database.Raw(
				`INSERT INTO comments (user_id, post_id, body, created_at, updated_at) VALUES ($1,$2,$3,$4,$5)`,
				commenterID, post.ID, body, createdAt, createdAt,
			).Exec()
			commentCount++
		}
	}
	fmt.Printf("  ✓  %d comments\n", commentCount)

	// ─── Notifications ───────────────────────────────────────────
	fmt.Println("🔔 Creating notifications...")
	// Like notifications
	type likeRow struct {
		PostID int64 `db:"post_id"`
		UserID int64 `db:"user_id"` // liker
	}
	var allLikes []likeRow
	_ = database.Raw(`SELECT post_id, user_id FROM likes LIMIT 30`).All(&allLikes)
	// map post→owner
	postOwner := map[int64]int64{}
	for _, p := range allPosts {
		postOwner[p.ID] = p.UserID
	}
	notifCount := 0
	for _, l := range allLikes {
		owner := postOwner[l.PostID]
		if owner == l.UserID {
			continue
		}
		_ = database.Raw(
			`INSERT INTO notifications (user_id, actor_id, type, post_id, read, created_at) VALUES ($1,$2,'like',$3,false,$4) ON CONFLICT DO NOTHING`,
			owner, l.UserID, l.PostID, time.Now().Add(-time.Duration(rand.Intn(300))*time.Minute),
		).Exec()
		notifCount++
	}
	// Follow notifications
	for _, pair := range followPairs[:10] {
		followerID := userIDs[pair[0]]
		followedID := userIDs[pair[1]]
		if followerID == 0 || followedID == 0 {
			continue
		}
		_ = database.Raw(
			`INSERT INTO notifications (user_id, actor_id, type, read, created_at) VALUES ($1,$2,'follow',false,$3) ON CONFLICT DO NOTHING`,
			followedID, followerID, time.Now().Add(-time.Duration(rand.Intn(480))*time.Minute),
		).Exec()
		notifCount++
	}
	fmt.Printf("  ✓  %d notifications\n", notifCount)

	// ─── Direct Messages ─────────────────────────────────────────
	fmt.Println("✉️  Creating DM threads...")
	for _, thread := range dmThreads {
		u1 := userIDs[thread.user1]
		u2 := userIDs[thread.user2]
		if u1 == 0 || u2 == 0 {
			continue
		}
		// Normalize pair (smaller id first)
		uid1, uid2 := u1, u2
		if uid1 > uid2 {
			uid1, uid2 = uid2, uid1
		}
		var convID int64
		err := database.Raw(
			`INSERT INTO conversations (user1_id, user2_id, created_at) VALUES ($1,$2,$3)
			 ON CONFLICT (user1_id, user2_id) DO UPDATE SET user1_id = EXCLUDED.user1_id RETURNING id`,
			uid1, uid2, time.Now().Add(-2*time.Hour),
		).Scan(&convID)
		if err != nil {
			fmt.Printf("  ⚠  conversation insert failed: %v\n", err)
			continue
		}
		lastTime := time.Now().Add(-time.Duration(len(thread.messages)+1) * 4 * time.Minute)
		for _, msg := range thread.messages {
			senderID := userIDs[msg.from]
			lastTime = lastTime.Add(4 * time.Minute)
			_ = database.Raw(
				`INSERT INTO messages (conversation_id, sender_id, body, read, created_at) VALUES ($1,$2,$3,true,$4)`,
				convID, senderID, msg.body, lastTime,
			).Exec()
		}
		_ = database.Raw(
			`UPDATE conversations SET last_message_at = $1 WHERE id = $2`,
			lastTime, convID,
		).Exec()
		fmt.Printf("  ✓  DM thread: @%s ↔ @%s (%d messages)\n", thread.user1, thread.user2, len(thread.messages))
	}

	fmt.Println("\n✅ Seeding complete!")
	fmt.Println("   Login with any account using password: password123")
	fmt.Println("   Example: alex@onigram.test / password123")
}

// ─── Helpers ─────────────────────────────────────────────────────

func downloadImage(url, dir, filename string) (string, error) {
	destPath := filepath.Join(dir, filename)
	// Skip if already downloaded
	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return "", err
	}
	return destPath, nil
}

var seedHashtagRe = regexp.MustCompile(`#(\w+)`)

// seedHashtags extracts hashtags from seeded post captions and links them.
func seedHashtags(contents []postContent, _ map[string]int64) {
	// Re-fetch all posts so we can match by caption prefix
	type postRow struct {
		ID      int64  `db:"id"`
		Caption string `db:"caption"`
	}
	var allPosts []postRow
	_ = database.Raw(`SELECT id, caption FROM posts`).All(&allPosts)

	tagged := 0
	for _, p := range allPosts {
		matches := seedHashtagRe.FindAllStringSubmatch(strings.ToLower(p.Caption), -1)
		seen := map[string]bool{}
		for _, m := range matches {
			tag := m[1]
			if seen[tag] {
				continue
			}
			seen[tag] = true
			_ = database.Raw(`INSERT INTO hashtags (tag) VALUES ($1) ON CONFLICT (tag) DO NOTHING`, tag).Exec()
			var hid int64
			if err := database.Raw(`SELECT id FROM hashtags WHERE tag = $1`, tag).Scan(&hid); err != nil || hid == 0 {
				continue
			}
			_ = database.Raw(`INSERT INTO post_hashtags (post_id, hashtag_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, p.ID, hid).Exec()
			tagged++
		}
	}
	_ = contents // unused, kept for call-site clarity
	fmt.Printf("  ✓  %d post-hashtag links created\n", tagged)
}

func must(err error, ctx string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s: %v\n", ctx, err)
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func keys(m map[string]int64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func shuffle(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
