import type { Article, Feed, FeedFolder } from "@/types/rss";

export const folders: FeedFolder[] = [
  { id: "dev", name: "Development", feedIds: ["hn", "golang", "vue"] },
  { id: "news", name: "News", feedIds: ["bbc", "verge"] },
];

export const feeds: Feed[] = [
  {
    id: "hn",
    title: "Hacker News",
    siteUrl: "https://news.ycombinator.com",
    feedUrl: "https://hnrss.org/frontpage",
    folderId: "dev",
    unreadCount: 12,
    lastFetchedAt: "2026-08-10T06:20:00Z",
  },
  {
    id: "golang",
    title: "The Go Blog",
    siteUrl: "https://go.dev/blog",
    feedUrl: "https://go.dev/blog/feed.atom",
    folderId: "dev",
    unreadCount: 3,
    lastFetchedAt: "2026-08-10T05:55:00Z",
  },
  {
    id: "vue",
    title: "The Vue Point",
    siteUrl: "https://blog.vuejs.org",
    feedUrl: "https://blog.vuejs.org/feed.rss",
    folderId: "dev",
    unreadCount: 1,
    lastFetchedAt: "2026-08-10T04:10:00Z",
  },
  {
    id: "bbc",
    title: "BBC World",
    siteUrl: "https://www.bbc.com/news",
    feedUrl: "https://feeds.bbci.co.uk/news/world/rss.xml",
    folderId: "news",
    unreadCount: 8,
    lastFetchedAt: "2026-08-10T06:30:00Z",
  },
  {
    id: "verge",
    title: "The Verge",
    siteUrl: "https://www.theverge.com",
    feedUrl: "https://www.theverge.com/rss/index.xml",
    folderId: "news",
    unreadCount: 5,
    lastFetchedAt: "2026-08-10T06:05:00Z",
  },
  {
    id: "apple",
    title: "Apple Newsroom",
    siteUrl: "https://www.apple.com/newsroom",
    feedUrl: "https://www.apple.com/newsroom/rss-feed.rss",
    unreadCount: 2,
    lastFetchedAt: "2026-08-10T03:40:00Z",
  },
];

const now = Date.now();
const hours = (h: number) => new Date(now - h * 3600_000).toISOString();
const days = (d: number) => new Date(now - d * 86_400_000).toISOString();

export const articles: Article[] = [
  {
    id: "a1",
    feedId: "hn",
    title: "Show HN: A calm desktop RSS reader built with Go and Vue",
    author: "hakup",
    summary:
      "An open-source reader focused on speed, offline-first storage, and a three-column layout inspired by Mail.",
    contentHtml: `
      <p>I wanted something lighter than a browser full of tabs, and quieter than most feed readers.</p>
      <p>This build uses a native shell, local SQLite, and a reading UI that stays out of the way. Unread, Today, and Starred sit up front; folders stay optional.</p>
      <h2>What matters</h2>
      <ul>
        <li>Open an article and keep reading without chrome noise</li>
        <li>Refresh in the background without modal interruptions</li>
        <li>Star what you want to revisit later</li>
      </ul>
      <p>If you try it, I’d love feedback on keyboard shortcuts and offline sync.</p>
    `,
    url: "https://news.ycombinator.com/item?id=1",
    publishedAt: hours(1),
    read: false,
    starred: true,
  },
  {
    id: "a2",
    feedId: "golang",
    title: "Go 1.26 is released",
    author: "The Go Team",
    summary:
      "Performance work continues, tooling improves, and the standard library gains a few carefully scoped additions.",
    contentHtml: `
      <p>Today we release Go 1.26, continuing the tradition of careful, backward-compatible evolution.</p>
      <p>Highlights include faster builds in large modules, refined garbage collector pacing, and better diagnostics for common concurrency mistakes.</p>
      <p>As always, please file issues if something breaks your real-world workflows.</p>
    `,
    url: "https://go.dev/blog/go1.26",
    publishedAt: hours(3),
    read: false,
    starred: false,
  },
  {
    id: "a3",
    feedId: "vue",
    title: "Announcing Vue 3.6",
    author: "Evan You",
    summary:
      "A maintenance-focused release with DX polish, better SSR hydration diagnostics, and smaller runtime chunks.",
    contentHtml: `
      <p>Vue 3.6 is primarily about stability and developer experience.</p>
      <p>We tightened types for script setup, improved warning messages, and continued shaving bytes from the production runtime.</p>
    `,
    url: "https://blog.vuejs.org/posts/vue-3-6",
    publishedAt: hours(6),
    read: false,
    starred: false,
  },
  {
    id: "a4",
    feedId: "bbc",
    title: "Markets steady as central banks hold rates",
    author: "BBC Business",
    summary:
      "Investors digested a fresh round of policy statements while energy prices eased slightly overnight.",
    contentHtml: `
      <p>Global markets traded cautiously after a cluster of central bank decisions left policy rates unchanged.</p>
      <p>Analysts said the pause reduced near-term volatility, though inflation data later this month could shift expectations again.</p>
    `,
    url: "https://www.bbc.com/news/business-1",
    publishedAt: hours(2),
    read: false,
    starred: false,
  },
  {
    id: "a5",
    feedId: "verge",
    title: "The best RSS apps in 2026",
    author: "The Verge Staff",
    summary:
      "From minimal readers to power-user dashboards, the feed ecosystem is healthier than it’s been in years.",
    contentHtml: `
      <p>Feeds never really died — they just left the mainstream conversation.</p>
      <p>This year we tested readers that prioritize calm layouts, solid offline support, and privacy-respecting sync.</p>
    `,
    url: "https://www.theverge.com/rss-apps-2026",
    publishedAt: hours(8),
    read: true,
    starred: true,
  },
  {
    id: "a6",
    feedId: "apple",
    title: "Apple introduces new accessibility features",
    author: "Apple Newsroom",
    summary:
      "Updates across iPhone, iPad, and Mac deepen support for vision, mobility, hearing, and cognitive accessibility.",
    contentHtml: `
      <p>Apple previewed a new set of accessibility features arriving later this year.</p>
      <p>The updates focus on personalization, clearer system feedback, and tools that help more people use devices independently.</p>
    `,
    url: "https://www.apple.com/newsroom/accessibility",
    publishedAt: days(1),
    read: false,
    starred: false,
  },
  {
    id: "a7",
    feedId: "hn",
    title: "Ask HN: How do you organize technical reading?",
    author: "reader42",
    summary:
      "Bookmarks rot. Tabs multiply. I’m looking for systems that keep signal high without becoming a second job.",
    contentHtml: `
      <p>I bounce between docs, papers, blogs, and changelogs. Most “read later” piles become graves.</p>
      <p>What actually works for you — folders, tags, time-boxed review, or something else?</p>
    `,
    url: "https://news.ycombinator.com/item?id=2",
    publishedAt: days(1),
    read: true,
    starred: false,
  },
  {
    id: "a8",
    feedId: "bbc",
    title: "Cities test quieter delivery corridors",
    summary:
      "Pilot programs restrict heavy traffic overnight and expand micro-hubs for last-mile bikes and vans.",
    contentHtml: `
      <p>Several cities are experimenting with quieter overnight logistics zones after resident complaints about noise and air quality.</p>
    `,
    url: "https://www.bbc.com/news/world-2",
    publishedAt: days(2),
    read: true,
    starred: false,
  },
  {
    id: "a9",
    feedId: "golang",
    title: "Profiling Go services without the drama",
    author: "Francesc Campoy",
    summary:
      "A practical path from “it’s slow” to a measured fix using pprof, benchmarks, and restraint.",
    contentHtml: `
      <p>Most performance work fails because it starts with intuition instead of evidence.</p>
      <p>Capture a baseline, isolate the hot path, and only then change code. The boring process wins.</p>
    `,
    url: "https://go.dev/blog/profiling",
    publishedAt: days(3),
    read: true,
    starred: true,
  },
  {
    id: "a10",
    feedId: "verge",
    title: "Why desktop apps still matter",
    author: "David Pierce",
    summary:
      "Browsers are powerful, but focused native shells remain unmatched for deep work and system integration.",
    contentHtml: `
      <p>Web apps are everywhere, and for good reason. Still, a well-made desktop app can feel calmer and more intentional.</p>
      <p>Keyboard-first navigation, offline storage, and OS-level integration remain hard to beat.</p>
    `,
    url: "https://www.theverge.com/desktop-apps",
    publishedAt: hours(12),
    read: false,
    starred: false,
  },
];
