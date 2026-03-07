(() => {
    const now = () => {
        return Math.floor(Date.now() / 1000);
    }

    const codeTtlMax = parseInt(document.querySelector("#code-ttl-max").value);
    const codeTtlStart = parseInt(document.querySelector("#code-ttl").value);

    // current unix time in seconds
    const startedAt = now();
    // timestamp when this code will expire
    const endsAt = startedAt + codeTtlStart;

    // fetch all spans that show time left
    const timeExpSpans = Array.from(document.querySelectorAll(".exp .exp-time"));
    // fetch foreground span
    const timeExpFg = document.querySelector(".exp.exp-fg");

    const secsUntilExpire = () => {
        return Math.max(endsAt - now(), 0);
    }

    const updateForegroundClipPath = () => {
        const ttlPct = 100 * secsUntilExpire() / codeTtlMax;
        timeExpFg.style.clipPath = `inset(0 ${100 - ttlPct}% 0 0)`;
    }
    
    const updateCountdown = () => {
        const exp = secsUntilExpire();

        const minsLeft = String(Math.floor(exp / 60));
        const secsLeft = String(exp % 60);

        const timeLeft = `${minsLeft}:${secsLeft.padStart(2, "0")}`;

        timeExpSpans.forEach(el => el.textContent = timeLeft);
    }
    
    updateForegroundClipPath();
    updateCountdown();

    setInterval(() => {
        // give a 1s leeway before reloading
        if (now() - endsAt > 0) {
            window.location.reload();
            return;
        }
        
        updateCountdown();
        updateForegroundClipPath();
    }, 1000);
})()
