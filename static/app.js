async function fetchAny(url) {
    const res = await fetch(url, { cache: "no-store" });
    const text = await res.text();

    try {
        return { ok: res.ok, status: res.status, data: JSON.parse(text) };
    } catch {
        return { ok: res.ok, status: res.status, data: text };
    }
}

function pretty(v) {
    return typeof v === "string" ? v : JSON.stringify(v, null, 2);
}

function byId(id) {
    const el = document.getElementById(id);
    if (!el) throw new Error(`Missing element with id="${id}"`);
    return el;
}

window.addEventListener("DOMContentLoaded", () => {
    const outHealth = byId("outHealth");
    const outTime = byId("outTime");
    const outStatic = byId("outStatic");

    byId("btnHealth").addEventListener("click", async () => {
        outHealth.textContent = "Loading...";
        const r = await fetchAny("/health");
        outHealth.textContent = pretty(r.data);
    });

    byId("btnTime").addEventListener("click", async () => {
        outTime.textContent = "Loading...";
        const r = await fetchAny("/api/time");
        outTime.textContent = pretty(r.data);
    });

    byId("btnStatic").addEventListener("click", async () => {
        outStatic.textContent = "Loading...";
        const r = await fetchAny("/static/hello.txt");
        outStatic.textContent = pretty(r.data);
    });
});
