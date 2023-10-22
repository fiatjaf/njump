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

// Needed to apply proper print styles
if (
  navigator.userAgent.indexOf('Safari') != -1 &&
  navigator.userAgent.indexOf('Chrome') == -1
) {
  document.body.classList.add('safari')
}
