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

// #todo_outline

= Introduction
We want to explore characteristics of the Mastodon network related to the geographical distribution of nodes, cloud providers nodes are hosted on, and more @mastodon. This paper is largely inspired by "Design and evaluation of IPFS: a storage layer for the decentralized web", which demonstrates the kinds of measurments that could be done on a decentralized network @ipfs.

= Methodology
While writing a crawler is simple in principle, to mitigate time constraints we began with a seed list of alive nodes created by an external, open-source Fediverse crawler @crawler. This crawler is updated every 6 hours. From the list created by this crawler, we filter down to Fediverse nodes hosting Mastodon or Mastodon-compatible software (as opposed to, for example, WordPress), and then to nodes which supported the `peers` API we leverage to determine node connectivity.

From this filtered list of nodes, we then collected data about each node leveraging the MaxMind GeoIP databases and by resolving the IPs for the domains, and looking up their ASN, AS organization names, country codes @maxmind. Furthermore, we use an `instance` API to collect user and post counts for all instances.

Upon collecting this data about nodes in a database, we use the neo4j graph database to insert all nodes, the properties of these nodes, and finally create `PEERS_WITH` relationships between peers as defined by the aforementioned `peers` API @neo4j @mastodon_peers.

We believe that this is a representative dataset of the Mastodon network, and thus feel that the analyses we conduct from this graph dataset represent the true network well. This is supported by the fact that statistics published by the Mastodon project state that there are 8739 instances and 9651558 users, which we are close to--especially considering that we count non-Mastodon software which supports the protocol @mastodon_statistics.

In the collection and analysis of this data, we created a Go program which connects to peers to ensure they are alive, determines whether their software is Mastodon API compatible, and then uses the `instance` API to collect user and post counts for all instances. This program further creates a number of SQLite databases and CSV files which are used for the analysis in this paper, directly or via neo4j. In addition, we created a number of Python scripts to process the data and create some of the visualizations, using Pandas, Mathplotlib, and GeoPandas. These are all available on GitHub @github.

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
// country,totalPosts,totalUsers,numberOfInstances,averagePostsPerUser
#let countries_most_avg = (
  csv("countries-with-highest-avg-posts-per-user.csv")
    .slice(1)
    .map(x => (
      cc_flag(x.at(0)),
      x.at(1),
      x.at(2),
      x.at(3),
      [#{ calc.round(float(x.at(4)), digits: 2) }],
    ))
)
#figure(
  caption: [Countries with Highest Average Posts per User],
  table(
    columns: (auto, auto, auto, auto, auto),
    align: (left, left, right, right, right),
    table.header[Country][Total Posts][Total Users][Number of Instances][Avg Posts per User],
    ..countries_most_avg.flatten(),
  ),
)

When looking at the countries with the highest average posts per user, we see that the Netherlands and Japan are far above the rest, with their average posts per user being approximately twice as high as the next highest country, South Korea. Furthermore, both of these countries have a meaningful number of instances, incidiating that this is not due to a few outliers.

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

Most instances are not hosted on cloud providers, and among those that are, surprisingly the most common are none of the big 3, but instead OVH, Hetzner, and DigitalOcean. In total, we see that #cloud_share of instances are hosted on cloud providers, indicating that the majority of instances are run on personal servers. This suggests that the Fediverse's decentralized nature extends beyond just its social structure to its infrastructure, with many operators choosing to maintain their own hardware rather than relying on major cloud providers.

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
      non_cloud_instances.map(x => x.at(0)),
      non_cloud_instances.map(x => x.at(4)),
      color: blue,
      label: [Non Cloud],
    ),
    lq.scatter(
      cloud_instances.map(x => x.at(0)),
      cloud_instances.map(x => x.at(4)),
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

The graph reveals that most Mastodon instances are hosted on high-ranking AS systems (higher numbers indicate lower influence in the global routing system), suggesting that the Fediverse infrastructure largely relies on smaller, less central network operators rather than the internet backbone providers. This aligns with the decentralized ethos of the Mastodon network, with many instances avoiding both major cloud providers and core internet infrastructure

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
#let n_most_peered_instances = 1000
#let most_peered_instances = (
  most_peered_instances
    .map(x => (x.at(0), int(x.at(1)), int(x.at(2)), int(x.at(3))))
    .filter(x => x.at(2) > 0 and x.at(3) > 0)
    .slice(0, n_most_peered_instances)
)
#let peers = most_peered_instances.map(x => x.at(1))
#let users = most_peered_instances.map(x => x.at(2))
#let posts = most_peered_instances.map(x => x.at(3))
#figure(
  caption: [Users and Posts vs Peers for #n_most_peered_instances Most Peered Instances],
  lq.diagram(
    lq.scatter(
      peers,
      users,
      color: blue,
      label: [Users],
    ),
    xlabel: "Peers",
    ylabel: "Users",
    // xscale: "log",
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


== Average Path Length
#let average_path_length = csv("average_path_length.csv").slice(1).flatten()
#figure(
  caption: [Average path length between any 2 points],
  table(
    columns: (auto, auto),
    align: (left, right),
    table.header[Statistic][Value],
    [Est. Avg Path Length], average_path_length.at(0),
    [Min Length], average_path_length.at(1),
    [Max Length], average_path_length.at(2),
    [Standard Deviation], average_path_length.at(3),
    [Pairs Considered], average_path_length.at(4),
  ),
)<average_path_length>

To estimate the Mastodon network’s average shortest‐path length, we randomly sample 1000 nodes of 9000 as source nodes. For each source, we run Dijkstra's and find the path length to every other node. Finally, we aggregate the sampled distances with `avg()`, `min()`, `max()`, and `stdev()` functions to yield estimates of the global mean, minimum, maximum, and standard deviation of path lengths. The result is an average path lenngth of 1.58 with standard deviation of 0.49.That’s exceptionally low, and most pairs are either directly peering or share exactly one intermediary. This indicates a very well connected network. With the same number of nodes and average number of degree, the Erdős–Rényi null model tells predicts the average path length between any 2 nodes to be around 1.13, effectively making this an almost complete graph. Due to how tightly connected this network is, we can also make the assumption peers aren't selected based on having similar content, since everyone is connected to one another.
