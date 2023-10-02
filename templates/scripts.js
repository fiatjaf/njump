const type = '{{.type}}'
let counts = []
let clients = document.querySelectorAll('.client')
for (let i = 0; i < clients.length; i++) {
  let name = clients[i].innerText
  let url = clients[i].href
  let key = 'nj:' + type + ':' + name
  let count = parseInt(localStorage.getItem(key) || 0)
  clients[i].parentNode.setAttribute('count', count)
  clients[i].parentNode.setAttribute('title', 'Used ' + count + ' times')
  clients[i].addEventListener('click', () => {
    localStorage.setItem(key, count + 1)
  })
  counts.push([count, name, url])
}

// Reorder clients following the counter
let clients_wrapper = document.querySelector('.clients_wrapper')
if (clients_wrapper !== null) {
  const elements = Array.from(clients_wrapper.getElementsByClassName('btn'))
  if (elements.length > 0) {
    elements.sort((a, b) => {
      const rankA = parseInt(a.getAttribute('count'))
      const rankB = parseInt(b.getAttribute('count'))
      return rankB - rankA
    })
    elements.forEach(element => clients_wrapper.appendChild(element))

    counts.sort((a, b) => b[0] - a[0])
  }
}

let jsons = document.querySelectorAll('.json')
for (let i = 0; i < jsons.length; i++) {
  jsons[i].innerHTML = syntaxHighlight(jsons[i].innerHTML)
}

const shareButton = document.querySelector('.open-list')
if (shareButton) {
  const clients_list = document.querySelector('.column_clients')
  shareButton.addEventListener('click', function () {
    clients_list.classList.toggle('up')
    if (clients_list.classList.contains('up')) {
      document.body.classList.add('lock')
    } else {
      document.body.classList.remove('lock')
    }
  })
}

function updateAdvanceSwitch() {
  advanced_list.forEach(element => {
    if (advanceSwitch.checked) {
      element.classList.add('visible')
    } else {
      element.classList.remove('visible')
    }
  })
}

const advanceSwitch = document.querySelector('.advanced-switch')
const advanced_list = document.querySelectorAll('.advanced')
if (advanceSwitch) {
  advanceSwitch.addEventListener('change', function () {
    updateAdvanceSwitch()
  })

  updateAdvanceSwitch() // Check at the page load, some browsers keep the state in cache
}

var url = new URL(window.location.href)
var searchParams = new URLSearchParams(url.search)
if (searchParams.has('details') && searchParams.get('details') == 'yes') {
  advanceSwitch.click()
}

function syntaxHighlight(json) {
  json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return json.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    function (match, p1) {
      var cls = 'number'
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          cls = 'key'
        } else {
          if (p1.length < 100) {
            cls = 'string'
          } else {
            cls = 'string content'
          }
        }
      } else if (/true|false/.test(match)) {
        cls = 'boolean'
      } else if (/null/.test(match)) {
        cls = 'null'
      }
      return '<span class="' + cls + '">' + match + '</span>'
    }
  )
}

function isElementInViewport(element) {
  // Get the position and dimensions of the element
  const rect = element.getBoundingClientRect()

  // Check if the element is within the viewport's boundaries
  return (
    rect.top >= 0 &&
    rect.left >= 0 &&
    rect.bottom <=
      (window.innerHeight || document.documentElement.clientHeight) &&
    rect.right <= (window.innerWidth || document.documentElement.clientWidth)
  )
}

document.addEventListener('DOMContentLoaded', function () {
  var contentDivs = document.getElementsByClassName('content')
  for (var i = 0; i < contentDivs.length; i++) {
    var contentDiv = contentDivs[i]
    if (contentDiv.offsetHeight == 160) {
      contentDiv.classList.add('gradient')
    }
  }
})

const desktop_name = document.querySelector('.column_content .name')
if (desktop_name) {
  window.addEventListener('scroll', function () {
    desktop_profile = document.querySelector('.column_content .info-wrapper')
    if (window.getComputedStyle(desktop_profile).display === 'none') {
      return
    }
    columnA = document.querySelector('.columnA')
    if (columnA != null && isElementInViewport(desktop_name)) {
      columnA.querySelector('.info-wrapper').style.display = 'none'
    } else {
      document.querySelector('.info-wrapper').style.display = 'block'
    }
  })
}

// Get all the npubs elements in last notes and link them
const headerDivs = document.querySelectorAll('div.header')
headerDivs.forEach(headerDiv => {
  const spanElements = headerDiv.querySelectorAll('span')
  spanElements.forEach(span => {
    const href = span.getAttribute('href')
    if (href) {
      span.addEventListener('click', ev => {
        ev.preventDefault()
        window.location.href = href
      })
    }
  })
})

// Needed to apply proper print styles
if (
  navigator.userAgent.indexOf('Safari') != -1 &&
  navigator.userAgent.indexOf('Chrome') == -1
) {
  document.body.classList.add('safari')
}
