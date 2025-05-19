#import "@preview/charged-ieee:0.1.3": ieee
#import "@preview/flagada:0.1.0" : *
#import "@preview/lilaq:0.3.0" as lq

#show: ieee.with(
  title: [Mastodon Network Analysis],
  abstract: [
The Mastodon network is a large area of research for sociologists and computer scientists alike, due to its distributed nature and unique audience. In this report, we crawl Mastodon network and measure statistics about the nature of the instances that make it up, and the connections between them. There exists research done on the content of the posts on Mastodon instances, but there is none to be found regarding the nature of the servers that run instances themselves, nor their connectivity with each other. 
  ],
  authors: (
    (
      name: "Ved Kothavade",
      // department: [Co-Founder],
      organization: [University of Maryland, College Park],
      location: [College Park, MD],
      email: "vedk@terpmail.umd.edu"
    ),
    (
      name: "Phan Ngyuen",
      // department: [Co-Founder],
      organization: [University of Maryland, College Park],
      location: [College Park, MD],
      email: "phan2003@terpmail.umd.edu"
    ),
  ),
  // index-terms: ("Scientific writing", "Typesetting", "Document creation", "Syntax"),
  // bibliography: bibliography("refs.bib"),
  figure-supplement: [Fig.],
)
#set table(align: (left, right), inset: (x: 8pt, y: 4pt), stroke: (x, y) => if y <= 1 { (top: 0.5pt) }, fill: (x, y) => if y > 0 and calc.rem(y, 2) == 0  { rgb("#efefef") })
#let cc_flag(x) = [#flag(x) #x]

= Introduction
We want to explore characteristics of the Mastodon network related to the geographical distribution of nodes, cloud providers nodes are hosted on, and more.

= Methodology
While writing a crawler is simple in principle, to mitigate time constraints we began with a seed list of alive nodes created by an external, open-source Fediverse crawler. This crawler is updated every 6 hours. From the list created by this crawler, we filter down to Fediverse nodes hosting Mastodon or Mastodon-compatible software (as opposed to, for example, WordPress), and then to nodes which supported the `peers` API we leverage to determine node connectivity.

From this filtered list of nodes, we then collected data about each node leveraging the MaxMind GeoIP databases and by resolving the IPs for the domains, and looking up their ASN, AS organization names, country codes. Furthermore, we use an `instance` API to collect user and post counts for all instances.

Upon collecting this data about nodes in a database, we use the neo4j graph database to insert all nodes, the properties of these nodes, and finally create `PEERS_WITH` relationships between peers as defined by the aforementioned `peers` API.

We believe that this is a representative dataset of the Mastodon network, and thus feel that the analyses we conduct from this graph dataset represent the true network well.

= Analysis
== Location of Instances
#figure(
  caption: [Heatmap of Instance Locations],
  // placement: top,
  image("map.png")
)
#let countries = csv("countries.csv")
#let top_n_countries = 10

#figure(
  caption: [Instance Counts in Top #top_n_countries Countries],
  // placement: top,
  table(
    columns: (6em, auto),
    table.header[Country][Count],
    ..countries.slice(0, top_n_countries).map(x => (cc_flag(x.at(0)), x.at(1))).flatten()
  )
)

== Cloud Providers
#let cloud_providers = csv("cloud_providers.csv")

#figure(
  caption: [Instance Counts by Cloud Provider],
  // placement: top,
  table(
    columns: (10em, auto),
    table.header[Cloud Provider][Count],
    ..cloud_providers.flatten()
  )
)

// none is first
// ovh hetzner digitalocean are 2-4
// then aws, gcp, azure
Most instances are not hosted on cloud providers, and among those that are, surprisingly the most common are none of the big 3, but instead OVH, Hetzner, and DigitalOcean.

#let asn_cloud_analysis = csv("asn_cloud_analysis.csv").slice(1)
#let cloud_instances = asn_cloud_analysis.filter(x => x.at(1) == "1")
#let non_cloud_instances = asn_cloud_analysis.filter(x => x.at(1) == "0")
#show: lq.set-legend(position: top + left)
#figure(
  caption: [Distribution of IPs across ASes according to their size (measured by their AS rank).],
  lq.diagram(
    lq.scatter(
      non_cloud_instances.map(x => int(x.at(0))),
      non_cloud_instances.map(x => int(x.at(2))),
      color: blue,
      label: [Non Cloud]
  ),
  lq.scatter(
    cloud_instances.map(x => int(x.at(0))),
    cloud_instances.map(x => int(x.at(2))),
    color: red,
    label: [Cloud]
  ),
  xscale: "log",
  yscale: "log",
  xlabel: "Autonomous System Rank",
  ylabel: "IP Address Count",
  title: "AS Distribution",
)
)