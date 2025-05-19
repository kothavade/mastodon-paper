import sqlite3
import pandas as pd
import geopandas
import matplotlib.pyplot as plt


db_path = "node_filter.db"
table_name = "node_info"
domain_col = "domain"
country_code_col = "country_code"

conn = sqlite3.connect(db_path)

query = f"SELECT {domain_col}, {country_code_col} FROM {table_name}"

df = pd.read_sql_query(query, conn)

country_counts = df[country_code_col].value_counts().reset_index()
country_counts.columns = ["country_code", "domain_count"]

print("\nDomain counts per country:")
print(country_counts.head())

world = geopandas.read_file("zip://110m_cultural.zip")
merged_data = world.merge(
    country_counts, left_on="ISO_A2", right_on="country_code", how="left"
)
merged_data["domain_count"] = merged_data["domain_count"].fillna(0)
fig, ax = plt.subplots(1, 1, figsize=(15, 10))

merged_data.plot(
    column="domain_count",
    ax=ax,
    legend=True,
    cmap="viridis",  # Colormap (e.g., 'OrRd', 'YlGnBu', 'viridis')
    legend_kwds={
        "orientation": "horizontal",
    },
)

ax.set_title("Number of Domains by Country", fontsize=24)
ax.set_axis_off()

# plt.show()
plt.savefig("paper/map.png", dpi=300)

country_counts.to_csv("paper/countries.csv", index=False, header=False)
