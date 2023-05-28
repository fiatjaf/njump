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
  elements.sort((a, b) => {
    const rankA = parseInt(a.getAttribute('count'))
    const rankB = parseInt(b.getAttribute('count'))
    return rankB - rankA
  })
  elements.forEach(element => clients_wrapper.appendChild(element))

  counts.sort((a, b) => b[0] - a[0])
  let tailsum = counts.slice(1).reduce((acc, c) => acc + c[0], 0)

  if (location.hash !== '#noredirect') {
    if (counts[0][0] - tailsum > 10) {
      location.href = counts[0][2]
    }
  }
}

let jsons = document.querySelectorAll('.json')
for (let i = 0; i < jsons.length; i++) {
  console.log(jsons[i].innerHTML)
  jsons[i].innerHTML = syntaxHighlight(jsons[i].innerHTML)
}

const shareButton = document.querySelector('.open-list')
const clients_list = document.querySelector('.column_clients')
shareButton.addEventListener('click', function () {
  clients_list.classList.toggle('up')
  if (clients_list.classList.contains('up')) {
    document.body.classList.add('lock')
  } else {
    document.body.classList.remove('lock')
  }
})

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
advanceSwitch.addEventListener('change', function () {
  updateAdvanceSwitch()
})

updateAdvanceSwitch() // Check at the page load, some browsers keep the state in cache

var url = new URL(window.location.href)
var searchParams = new URLSearchParams(url.search)
if (searchParams.has('details') && searchParams.get('details') == 'yes') {
  advanceSwitch.click()
}

function syntaxHighlight(json) {
  json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return json.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    function (match) {
      var cls = 'number'
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          cls = 'key'
        } else {
          cls = 'string'
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
