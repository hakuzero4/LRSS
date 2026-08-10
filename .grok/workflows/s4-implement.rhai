let meta = #{
    name: "s4-implement",
    description: "S4 multi-agent: OPML + folders + auto-refresh per docs/plan-s4.md",
    when_to_use: "Implement or finish LRSS S4 (OPML, folders, auto-refresh) with parallel ownership agents",
    phases: [
        #{ title: "Leaf packages", detail: "opml package + repo/service folder CRUD" },
        #{ title: "Integration", detail: "Import/Export API, auto-refresh, frontend" },
        #{ title: "Verify", detail: "go test and plan checklist" },
    ],
};

phase("Leaf packages");
log("Phase 1: parallel Agent A (opml) + Agent B (folders)");

let phase1 = parallel([
    #{
        label: "s4-A-opml",
        capability_mode: "all",
        prompt: "LRSS at C:/projects/temp/LRSS. Agent A: implement ONLY internal/opml (parse+export+tests) per docs/plan-s4.md. API: Parse([]byte)(*Document,error), Export(*Document)([]byte,error). No other packages. Run go test ./internal/opml/...",
    },
    #{
        label: "s4-B-folders",
        capability_mode: "all",
        prompt: "LRSS at C:/projects/temp/LRSS. Agent B: extend repo folder/feed + service Folder CRUD, MoveFeed, SetPaused per docs/plan-s4.md. Edit only internal/repo (folder/feed/tests), internal/service/ports.go and folder-related service methods/tests. NO opml, NO ImportOPML, NO appsvc, NO main, NO frontend. Run go test ./internal/repo/... ./internal/service/...",
    },
]);

for r in phase1 {
    if r == () || !r.success {
        log("Phase 1 agent failed or empty; continuing with remaining work");
    }
}

phase("Integration");
log("Phase 2: parallel Agent C (API+ticker) + Agent D (frontend)");

let phase2 = parallel([
    #{
        label: "s4-C-api",
        capability_mode: "all",
        prompt: "LRSS at C:/projects/temp/LRSS. Agent C: wire ImportOPML/ExportOPML using internal/opml, LibraryConfig settings, appsvc FeedService+Settings methods, main.go auto-refresh mutex per docs/plan-s4.md. Use existing Library folder methods if present. Do not edit frontend or reimplement opml parse. Run go test ./internal/...",
    },
    #{
        label: "s4-D-ui",
        capability_mode: "all",
        prompt: "LRSS at C:/projects/temp/LRSS. Agent D: frontend only — FeedsPanel OPML import/export, GeneralPanel LibraryConfig persistence, useRssStore createFolder/importOPML/exportOPML per docs/plan-s4.md. Call FeedService/SettingsService methods when present; guard missing bindings. No Go edits.",
    },
]);

for r in phase2 {
    if r == () || !r.success {
        log("Phase 2 agent issue noted");
    }
}

phase("Verify");
let verify = agent(
    "LRSS at C:/projects/temp/LRSS. Run go test ./... and report pass/fail. Check docs/plan-s4.md acceptance items against code (grep ImportOPML ExportOPML CreateFolder AutoRefresh LibraryConfig). List any gaps; tick what is done in plan if you can edit docs. Do not claim complete without tests green.",
    #{ label: "s4-verify", capability_mode: "all" },
);

if verify != () && verify.success {
    complete(#{ summary: "S4 multi-agent run finished; see verify agent output", ok: true });
}
complete(#{ summary: "S4 multi-agent run finished with gaps", ok: false });
