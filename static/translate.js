// Translate only the nostr post body (the [itemprop="articleBody"] element)
// into the visitor's browser language.
// The request is proxied through njump's own /njump/translate endpoint, which
// forwards it to the configured translation backend (keeping any API key secret
// and avoiding CORS). Self-contained: injects its own styles so no Tailwind
// rebuild is needed.
(function () {
  var style = document.createElement('style')
  style.textContent =
    '.njump-translate-btn{background:none;border:none;padding:0;cursor:pointer;' +
    'font:inherit;font-size:0.8rem;color:#a8a29e;text-decoration:underline;' +
    'text-underline-offset:2px;}' +
    '.njump-translate-btn:hover{color:#e32a6d;}' +
    '.njump-translate-btn[disabled]{cursor:default;opacity:0.6;}' +
    '.njump-translation{margin-top:0.75rem;border-left:3px solid #e32a6d;' +
    'padding-left:0.75rem;white-space:pre-wrap;line-height:1.5rem;}'
  document.head.appendChild(style)

  function targetLang() {
    var l = navigator.language || navigator.userLanguage || 'en'
    return l.split('-')[0].toLowerCase()
  }

  function translate(text, target) {
    return fetch('/njump/translate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ q: text, target: target }),
    })
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status)
        return r.json()
      })
      .then(function (data) {
        return (data && data.translatedText) || ''
      })
  }

  document.addEventListener('click', function (ev) {
    var btn = ev.target.closest && ev.target.closest('.njump-translate-btn')
    if (!btn) return
    ev.preventDefault()

    var content = document.querySelector('[itemprop="articleBody"]')
    if (!content) return

    var existing = document.querySelector('.njump-translation')
    if (existing) {
      existing.remove()
      btn.textContent = btn.getAttribute('data-label-show')
      return
    }

    var text = (content.innerText || content.textContent || '').trim()
    if (!text) return

    btn.disabled = true
    btn.textContent = btn.getAttribute('data-label-loading') || '…'

    translate(text, targetLang())
      .then(function (translated) {
        var box = document.createElement('div')
        box.className = 'njump-translation'
        box.lang = targetLang()
        box.setAttribute('dir', 'auto')
        box.textContent = translated
        content.insertAdjacentElement('afterend', box)
        btn.textContent = btn.getAttribute('data-label-hide') || 'Show original'
      })
      .catch(function (err) {
        console.error('njump translate:', err)
        btn.textContent = btn.getAttribute('data-label-error') || 'Translation failed'
        window.setTimeout(function () {
          btn.textContent = btn.getAttribute('data-label-show')
        }, 2500)
      })
      .finally(function () {
        btn.disabled = false
      })
  })
})()
