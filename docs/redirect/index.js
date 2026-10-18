// Redirects every other effeck.* hostname to the docs on effeck.dev,
// keeping the path and query string.
export default {
  fetch(request) {
    const url = new URL(request.url)
    return Response.redirect(`https://effeck.dev${url.pathname}${url.search}`, 301)
  }
}
