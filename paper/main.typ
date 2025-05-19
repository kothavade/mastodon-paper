#import "@preview/charged-ieee:0.1.3": ieee
#import "@preview/flagada:0.1.0": *
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
      email: "vedk@terpmail.umd.edu",
    ),
    (
      name: "Phan Ngyuen",
      // department: [Co-Founder],
      organization: [University of Maryland, College Park],
      location: [College Park, MD],
      email: "phan2003@terpmail.umd.edu",
    ),
  ),
  // index-terms: ("Scientific writing", "Typesetting", "Document creation", "Syntax"),
  bibliography: bibliography("refs.bib"),
  figure-supplement: [Fig.],
)
#set table(
  align: (left, right),
  inset: (x: 8pt, y: 4pt),
  stroke: (x, y) => if y <= 1 { (top: 0.5pt) },
  fill: (x, y) => if y > 0 and calc.rem(y, 2) == 0 { rgb("#efefef") },
)
#let cc_flag(x) = [#flag(x) #x]
#let pct(x) = [#{ calc.round(x * 100, digits: 2) }%]


= Introduction
We want to explore characteristics of the Mastodon network related to the geographical distribution of nodes, cloud providers nodes are hosted on, and more. This paper is largely inspired by "Design and evaluation of IPFS: a storage layer for the decentralized web", which demonstrates the kinds of measurments that could be done on a decentralized network @ipfs.

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
  image("map.png"),
)<map>
#let countries = csv("countries.csv")
#let top_n_countries = 10
#let total_instances = countries.map(x => int(x.at(1))).sum()
#let countries_with_share = (
  countries
    .slice(0, top_n_countries)
    .enumerate()
    .map(x => (
      [#{ int(x.at(0)) + 1 }],
      cc_flag(x.at(1).at(0)),
      x.at(1).at(1),
      [#{ pct(int(x.at(1).at(1)) / total_instances) }],
    ))
)
#figure(
  caption: [Instance Counts in Top #top_n_countries Countries],
  // placement: top,
  table(
    align: (left, left, right, right),
    columns: (auto, 6em, auto, auto),
    table.header[Rank][Country][Count][Share],
    ..countries_with_share.flatten(),
    table.footer[Total][][#total_instances][],
  ),
)<country_table>

#let top_5_total = countries.slice(0, 5).map(x => int(x.at(1))).sum()
#let top_5_share = pct(top_5_total / total_instances)

As @country_table shows, the majority of instances are in the United States, followed by France, Germany, Portugal, and Japan. In fact, the top 5 countries alone account for #top_5_share of all instances.

== Cloud Providers
#let cloud_providers = csv("cloud_providers.csv")
#let total_instances = cloud_providers.map(x => int(x.at(1))).sum()
#let ranked_cloud_providers = (
  cloud_providers
    .enumerate()
    .map(x => (
      [#{ int(x.at(0)) + 1 }],
      x.at(1).at(0),
      x.at(1).at(1),
      [#{ pct(int(x.at(1).at(1)) / total_instances) }],
    ))
)

#figure(
  caption: [Instance Counts by Cloud Provider],
  // placement: top,
  table(
    columns: (auto, 10em, auto, auto),
    align: (left, left, right, right),
    table.header[Rank][Cloud Provider][Count][Share],
    ..ranked_cloud_providers.flatten(),
    table.footer[Total][][#total_instances][],
  ),
)

// none is first
// ovh hetzner digitalocean are 2-4
// then aws, gcp, azure
Most instances are not hosted on cloud providers, and among those that are, surprisingly the most common are none of the big 3, but instead OVH, Hetzner, and DigitalOcean. In total, we see that 46.72% of instances are hosted on cloud providers.

#let asn_cloud_analysis = csv("asn_cloud_analysis.csv").slice(1)
#let cloud_instances = asn_cloud_analysis.filter(x => x.at(2) == "1")
#let non_cloud_instances = asn_cloud_analysis.filter(x => x.at(2) == "0")
#show: lq.set-legend(position: top + left)
#figure(
  caption: [Distribution of IPs across ASes according to their size (measured by their AS rank).],
  lq.diagram(
    lq.scatter(
      non_cloud_instances.map(x => int(x.at(0))),
      non_cloud_instances.map(x => int(x.at(3))),
      color: blue,
      label: [Non Cloud],
    ),
    lq.scatter(
      cloud_instances.map(x => int(x.at(0))),
      cloud_instances.map(x => int(x.at(3))),
      color: red,
      label: [Cloud],
    ),
    xscale: "log",
    yscale: "log",
    xlabel: "Autonomous System Rank",
    ylabel: "IP Address Count",
    title: "AS Distribution",
  ),
)

Using CAIDA's AS Rank, we ranked... @as_rank.
