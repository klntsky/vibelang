// All effeck.* domains permanently redirect to the single VibeLang site.
export default {
  fetch(request) {
    const url = new URL(request.url)
    const path = url.pathname === "/index.html" ? "/" : url.pathname.replace(/\.html$/, "")
    return Response.redirect("https://vibelang.sh" + path + url.search, 301)
  }
}
