#import "@preview/charged-ieee:0.1.3": ieee
#import "@preview/flagada:0.1.0": *
#import "@preview/lilaq:0.3.0" as lq
#import "@preview/big-todo:0.2.0": *
#show: ieee.with(
  title: [Mastodon Network Analysis],
  abstract: [
    The Mastodon network is a large area of research for sociologists and computer scientists alike, due to its distributed nature and unique audience. In this report, we crawl Mastodon network and measure statistics about the nature of the instances that make it up, and the connections between them. There exists research done on the content of the posts on Mastodon instances, but there is none to be found regarding the nature of the servers that run instances themselves, nor their connectivity with each other.
  ],
  authors: (
    (
      name: "Ved Kothavade",
      organization: [University of Maryland, College Park],
      location: [College Park, MD],
      email: "vedk@terpmail.umd.edu",
    ),
    (
      name: "Phan Ngyuen",
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

#todo_outline

= Introduction
We want to explore characteristics of the Mastodon network related to the geographical distribution of nodes, cloud providers nodes are hosted on, and more @mastodon. This paper is largely inspired by "Design and evaluation of IPFS: a storage layer for the decentralized web", which demonstrates the kinds of measurments that could be done on a decentralized network @ipfs.

= Methodology
While writing a crawler is simple in principle, to mitigate time constraints we began with a seed list of alive nodes created by an external, open-source Fediverse crawler @crawler. This crawler is updated every 6 hours. From the list created by this crawler, we filter down to Fediverse nodes hosting Mastodon or Mastodon-compatible software (as opposed to, for example, WordPress), and then to nodes which supported the `peers` API we leverage to determine node connectivity.

From this filtered list of nodes, we then collected data about each node leveraging the MaxMind GeoIP databases and by resolving the IPs for the domains, and looking up their ASN, AS organization names, country codes @maxmind. Furthermore, we use an `instance` API to collect user and post counts for all instances.

Upon collecting this data about nodes in a database, we use the neo4j graph database to insert all nodes, the properties of these nodes, and finally create `PEERS_WITH` relationships between peers as defined by the aforementioned `peers` API @neo4j @mastodon_peers.

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
    columns: (auto, auto, auto, auto),
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
    columns: (auto, auto, auto, auto),
    align: (left, left, right, right),
    table.header[Rank][Cloud Provider][Count][Share],
    ..ranked_cloud_providers.flatten(),
    table.footer[Total][][#total_instances][],
  ),
)

#let total_cloud_instances = cloud_providers.slice(1).map(x => int(x.at(1))).sum()
#let cloud_share = pct(total_cloud_instances / total_instances)

Most instances are not hosted on cloud providers, and among those that are, surprisingly the most common are none of the big 3, but instead OVH, Hetzner, and DigitalOcean. In total, we see that #cloud_share of instances are hosted on cloud providers, indicating that the majority of instances are run on personal servers. From this, we can infer that many Mastodon administrators are #todo([smth about relevance of non-cloud], inline: true)

== Autonomous Systems

#let asn_cloud_analysis = (
  csv("asn_cloud_analysis.csv").slice(1).map(x => (int(x.at(0)), int(x.at(1)), x.at(2), int(x.at(3)), int(x.at(4))))
)
#let cloud_instances = asn_cloud_analysis.filter(x => x.at(3) == 1)
#let non_cloud_instances = asn_cloud_analysis.filter(x => x.at(3) == 0)
#figure(
  caption: [Distribution of IPs across ASes according to their size (measured by their AS rank).],
  lq.diagram(
    lq.scatter(
      non_cloud_instances.map(x => int(x.at(0))),
      non_cloud_instances.map(x => int(x.at(4))),
      color: blue,
      label: [Non Cloud],
    ),
    lq.scatter(
      cloud_instances.map(x => int(x.at(0))),
      cloud_instances.map(x => int(x.at(4))),
      color: red,
      label: [Cloud],
    ),
    xscale: "log",
    yscale: "log",
    xlabel: "Autonomous System Rank",
    ylabel: "IP Address Count",
    title: "AS Distribution",
    legend: (position: left + top),
  ),
)<as_distribution>

We mapped IP addresses to Autonomous Systems using GeoLite2, found the CAIDA AS Rank for each AS. According to CAIDA, the AS Rank is a measure of the influcence of an AS to the global routing system, as calculated by their sizes, peering agreements, and more @as_rank.

We then plotted the number of IP addresses per AS, colored by whether the AS is a cloud provider or not. We see that the cloud providers are disproportionately large, and that there are many more IP addresses in the cloud providers than in the non-cloud providers.

#let n_top_as = 5
#let top_as_names = (
  asn_cloud_analysis
    .sorted(key: x => int(x.at(4)))
    .rev()
    .slice(0, n_top_as)
    .map(x => (
      [#x.at(0)],
      x.at(2),
      [#x.at(1)],
      [#x.at(4)],
      [#{ pct(int(x.at(4)) / total_instances) }],
    ))
)
#let top_as_n_share = (
  asn_cloud_analysis.sorted(key: x => x.at(4)).rev().slice(0, n_top_as).map(x => x.at(4)).sum()
)
#let top_as_n_share = pct(top_as_n_share / total_instances)
#figure(
  caption: [Autonomous Systems covering $>50%$ of all found IP Addresses],
  table(
    columns: (auto, auto, auto, auto, auto),
    align: (left, left, right, right, right),
    table.header[ASN][AS Rank][AS Name][Count][Share],
    ..top_as_names.flatten(),
    table.footer[Total][][][#total_instances][#top_as_n_share],
  ),
)



== Most Peered Instances
#let most_peered_instances = csv("most-peered-instances.csv").slice(1)
#figure(
  caption: [Top 10 Most Peered Instances],
  table(
    columns: (auto, auto, auto, auto),
    align: (left, right, right, right),
    table.header[Instance][Peers][Users][Posts],
    ..most_peered_instances.slice(0, 11).flatten(),
  ),
)<most_peered_instances>

From @most_peered_instances, we see that the most peered instance is `mastodon.social`, with #most_peered_instances.at(1).at(1) peers. There appears to be a trend that the more peered an instance is, the more users and posts it has, but this is not strict.

#let most_peered_instances = (
  most_peered_instances
    .map(x => (x.at(0), int(x.at(1)), int(x.at(2)), int(x.at(3))))
    .filter(x => x.at(2) > 0 and x.at(3) > 0)
    .slice(0, 1000)
)
#let peers = most_peered_instances.map(x => x.at(1))
#let users = most_peered_instances.map(x => x.at(2))
#let posts = most_peered_instances.map(x => x.at(3))
#figure(
  caption: [Users and Posts vs Peers for 1000 Most Peered Instances],
  lq.diagram(
    lq.scatter(
      peers,
      users,
      color: blue,
      label: [Users],
    ),
    xlabel: "Peers",
    ylabel: "Users",
    xscale: "log",
    yscale: "log",
    lq.yaxis(
      lq.scatter(
        peers,
        posts,
        color: red,
        label: [Posts],
      ),
      position: right,
      label: [Posts],
      scale: "log",
    ),
    legend: (position: left + top),
    title: "Users and Posts vs Peers",
  ),
)<peers_users_posts>

We see that there is a positive correlation between the number of peers and the number of users and posts in @peers_users_posts.
