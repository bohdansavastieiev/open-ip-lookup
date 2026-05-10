const IP_KIND = Object.freeze({
	routable: "Routable",
	specialUse: "Special Use",
	unallocated: "Unallocated",
});

const SORT_DIRECTION = Object.freeze({
	asc: "asc",
	desc: "desc",
});

const TABLE_COLUMNS = Object.freeze([
	{ key: "ip", label: "IP", sortable: true },
	{ key: "occurrences", label: "Seen", sortable: true },
	{ key: "country", label: "Country", sortable: true },
	{ key: "region", label: "Region", sortable: true },
	{ key: "city", label: "City", sortable: true },
	{ key: "asn", label: "ASN", sortable: true },
	{ key: "organization", label: "Organization", sortable: true },
	{ key: "flags", label: "Flags", sortable: true },
]);

const FLAG_FILTER_KEY = "__flags";

const FILTER_GROUPS = Object.freeze([
	{ key: "family", label: "IP version" },
	{ key: "kind", label: "Kind" },
	{ key: "country", label: "Country" },
	{ key: "region", label: "Region" },
	{ key: "city", label: "City" },
	{ key: "asn", label: "ASN" },
	{ key: "organization", label: "Organization" },
]);

const form = document.querySelector("[data-lookup-form]");
const inputNode = document.querySelector("[data-lookup-input]");
const inputSizeNode = document.querySelector("[data-input-size]");
const statusNode = document.querySelector("[data-lookup-status]");
const controlsNode = document.querySelector("[data-lookup-controls]");
const resultsNode = document.querySelector("[data-lookup-results]");
const submitButton = document.querySelector("[data-submit-button]");
const textEncoder = new TextEncoder();
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

const state = {
	report: null,
	rows: [],
	sort: null,
	filters: new Map(),
	flagFilters: new Set(),
	expandedRows: new Set(),
	openFilterKey: null,
	isBusy: false,
};

form.addEventListener("submit", handleLookupSubmit);
inputNode.addEventListener("input", updateFormState);
controlsNode.addEventListener("change", handleControlsChange);
controlsNode.addEventListener("click", handleControlsClick);
resultsNode.addEventListener("click", handleResultsClick);
document.addEventListener("click", handleDocumentClick);

renderInitialState();
updateFormState();

async function handleLookupSubmit(event) {
	event.preventDefault();

	if (inputNode.value.trim() === "") {
		showError("Enter IPs or text containing IPs.");
		inputNode.focus();
		return;
	}

	const body = encodedFormBody(form);
	const maxBodyBytes = maxLookupBodyBytes();
	if (byteLength(body) > maxBodyBytes) {
		showError(`Input is too large. Limit: ${formatBytes(maxBodyBytes)}.`);
		return;
	}

	setBusy(true);
	hideStatus();
	try {
		const report = await lookup(body);
		loadReport(report);
		renderApp();
		hideStatus();
		scrollToLookupOutput();
	} catch (err) {
		showError(err.message);
	} finally {
		setBusy(false);
	}
}

function handleControlsChange(event) {
	const filterValue = event.target.dataset.filterValue;
	if (filterValue !== undefined) {
		toggleFilterValue(
			event.target.dataset.filterKey,
			filterValue,
			event.target.checked,
		);
		return;
	}

	const flagValue = event.target.dataset.flagFilter;
	if (flagValue !== undefined) {
		toggleFlagFilter(flagValue, event.target.checked);
		return;
	}
}

function handleControlsClick(event) {
	const clearFiltersButton = event.target.closest("[data-clear-filters]");
	if (clearFiltersButton) {
		clearFilters();
		return;
	}

	const clearFilterButton = event.target.closest("[data-clear-filter]");
	if (clearFilterButton) {
		clearFilter(clearFilterButton.dataset.clearFilter);
		return;
	}

	const clearFilterValueButton = event.target.closest("[data-clear-filter-value]");
	if (clearFilterValueButton) {
		clearFilterValue(
			clearFilterValueButton.dataset.clearFilterValue,
			clearFilterValueButton.dataset.value,
		);
		return;
	}

	const clearFlagValueButton = event.target.closest("[data-clear-flag-value]");
	if (clearFlagValueButton) {
		clearFlagValue(clearFlagValueButton.dataset.clearFlagValue);
		return;
	}

	const filterButton = event.target.closest("[data-filter-menu]");
	if (filterButton) {
		toggleFilterMenu(filterButton.dataset.filterMenu);
		return;
	}

}

function handleDocumentClick(event) {
	if (!state.openFilterKey || event.target.closest("[data-menu-shell]")) {
		return;
	}
	state.openFilterKey = null;
	renderApp();
}

function handleResultsClick(event) {
	const sortButton = event.target.closest("[data-sort-key]");
	if (sortButton) {
		cycleSort(sortButton.dataset.sortKey);
		return;
	}

	const expandButton = event.target.closest("[data-expand-row]");
	if (expandButton) {
		toggleExpandedRow(Number(expandButton.dataset.expandRow));
	}
}

function encodedFormBody(formNode) {
	return new URLSearchParams(new FormData(formNode)).toString();
}

function byteLength(value) {
	return textEncoder.encode(value).length;
}

function maxLookupBodyBytes() {
	return Number(form.dataset.maxBodyBytes);
}

async function lookup(body) {
	const response = await fetch(form.action, {
		method: "POST",
		headers: { "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8" },
		body,
	});

	if (!response.ok) {
		throw new Error(await responseErrorMessage(response));
	}

	return response.json();
}

async function responseErrorMessage(response) {
	const fallback = `Lookup failed with status ${response.status}.`;
	if (!response.headers.get("Content-Type")?.includes("application/json")) {
		return fallback;
	}

	const data = await response.json();
	return data.error || fallback;
}

function loadReport(report) {
	state.report = report;
	state.rows = normalizeReport(report);
	state.sort = null;
	state.filters = new Map();
	state.flagFilters = new Set();
	state.expandedRows = new Set();
	state.openFilterKey = null;
}

function normalizeReport(report) {
	return report.entries.map((entry, index) => normalizeEntry(entry, index));
}

function normalizeEntry(entry, index) {
	const flags = entry.flags || [];
	return {
		index,
		entry,
		ip: entry.ip,
		occurrences: entry.occurrences,
		ipSortKey: parseIPSortKey(entry.ip),
		family: entry.isIpv6 ? "IPv6" : "IPv4",
		kind: value(entry.kind),
		country: value(entry.geo?.country),
		countryEmoji: value(entry.geo?.countryEmoji),
		countryIso: value(entry.geo?.countryIso),
		region: value(entry.geo?.region),
		city: value(entry.geo?.city),
		asn: asnLabel(entry.asn),
		asnNumber: entry.asn?.number || 0,
		organization: value(entry.asn?.organization),
		flags,
		flagSet: new Set(flags),
	};
}

function asnLabel(asn) {
	if (!asn?.number) {
		return "";
	}
	return `AS${asn.number}`;
}

function value(raw) {
	return raw || "";
}

function parseIPSortKey(ip) {
	if (ip.includes(":")) {
		return { family: 6, value: parseIPv6(ip) };
	}
	return { family: 4, value: parseIPv4(ip) };
}

function parseIPv4(ip) {
	return ip.split(".").reduce((value, octet) => {
		return (value << 8n) + BigInt(Number(octet));
	}, 0n);
}

function parseIPv6(ip) {
	const [head, tail = ""] = ip.split("::");
	const headParts = parseIPv6Part(head);
	const tailParts = parseIPv6Part(tail);
	const missingParts = 8 - headParts.length - tailParts.length;
	const parts = headParts.concat(Array(missingParts).fill(0), tailParts);

	return parts.reduce((value, part) => {
		return (value << 16n) + BigInt(part);
	}, 0n);
}

function parseIPv6Part(part) {
	if (!part) {
		return [];
	}

	const pieces = part.split(":");
	const last = pieces[pieces.length - 1];
	if (!last.includes(".")) {
		return pieces.map((piece) => Number.parseInt(piece, 16));
	}

	return pieces.slice(0, -1).map((piece) => Number.parseInt(piece, 16))
		.concat(parseIPv4Hextets(last));
}

function parseIPv4Hextets(ip) {
	const value = parseIPv4(ip);
	return [
		Number((value >> 16n) & 0xffffn),
		Number(value & 0xffffn),
	];
}

function renderInitialState() {
	hideStatus();
	controlsNode.hidden = true;
	resultsNode.replaceChildren(emptyState("Results will appear here", "", "quiet"));
}

function renderApp() {
	const filteredRows = filterRows(state.rows);
	const visibleRows = sortRows(filteredRows, state.sort);
	renderControls(state.rows);
	renderResults(visibleRows, filteredRows.length);
}

function plural(count, singular) {
	return count === 1 ? singular : `${singular}s`;
}

function renderControls(rows) {
	controlsNode.replaceChildren();
	controlsNode.hidden = true;

	if (rows.length === 0) {
		return;
	}

	const filters = document.createElement("div");
	filters.className = "filter-list";

	for (const group of FILTER_GROUPS) {
		const options = filterOptions(rows, group.key);
		const selectedCount = state.filters.get(group.key)?.size || 0;
		if (hasUsefulFilterOptions(options) || selectedCount > 0) {
			filters.appendChild(renderFilterGroup(group, options));
		}
	}

	const flags = flagOptions(rows);
	if (hasUsefulFilterOptions(flags) || state.flagFilters.size > 0) {
		filters.appendChild(renderFlagFilterGroup(flags));
	}

	const activeFilters = renderActiveFilters();

	if (filters.children.length === 0 && !activeFilters) {
		state.openFilterKey = null;
		return;
	}

	controlsNode.hidden = false;
	controlsNode.className = "controls-panel";

	const filterSection = document.createElement("section");
	filterSection.className = "filter-section";
	filterSection.appendChild(renderFilterHeader());

	if (filters.children.length > 0) {
		filterSection.appendChild(filters);
	}

	if (activeFilters) {
		filterSection.appendChild(activeFilters);
	}

	controlsNode.appendChild(filterSection);
}

function sectionHeading(text) {
	const heading = document.createElement("h2");
	heading.className = "section-heading";
	heading.textContent = text;
	return heading;
}

function hasUsefulFilterOptions(options) {
	return options.length > 1;
}

function renderFilterHeader() {
	const wrapper = document.createElement("div");
	wrapper.className = "filter-header";

	const heading = sectionHeading("Filters");
	const actions = document.createElement("div");
	actions.className = "filter-actions";

	const activeFilters = activeFilterCount();
	const clearButton = buttonElement(
		`Clear filters${activeFilters ? ` (${activeFilters})` : ""}`,
		"button-ghost",
	);
	clearButton.dataset.clearFilters = "true";
	clearButton.disabled = activeFilters === 0;

	actions.appendChild(clearButton);
	wrapper.append(heading, actions);
	return wrapper;
}

function renderActiveFilters() {
	const chips = activeFilterChips();
	if (chips.length === 0) {
		return null;
	}

	const wrapper = document.createElement("div");
	wrapper.className = "active-filters-panel";

	const list = document.createElement("div");
	list.className = "active-filters-list";
	for (const chip of chips) {
		list.appendChild(chip);
	}
	wrapper.appendChild(list);
	return wrapper;
}

function activeFilterChips() {
	const chips = [];
	for (const group of FILTER_GROUPS) {
		const values = state.filters.get(group.key);
		if (!values) {
			continue;
		}
		for (const value of values) {
			chips.push(activeFilterChip(group.label, value, group.key));
		}
	}

	for (const flag of state.flagFilters) {
		chips.push(activeFlagChip(flag));
	}
	return chips;
}

function activeFilterChip(label, value, key) {
	const valueLabel = filterOptionLabel(value, label);
	const text = `${label}: ${valueLabel}`;
	const chip = chipShell(label, valueLabel);
	chip.appendChild(chipClearButton("data-clear-filter-value", key, value, `Remove ${text}`));
	return chip;
}

function activeFlagChip(flag) {
	const valueLabel = filterOptionLabel(flag, "Flags");
	const text = `Flags: ${valueLabel}`;
	const chip = chipShell("Flags", valueLabel);
	chip.appendChild(chipClearButton("data-clear-flag-value", flag, undefined, `Remove ${text}`));
	return chip;
}

function chipShell(label, valueLabel) {
	const chip = document.createElement("span");
	chip.className = "filter-chip";

	const labelSpan = document.createElement("span");
	labelSpan.className = "filter-chip-label";
	labelSpan.textContent = `${label}:`;

	const valueSpan = document.createElement("span");
	valueSpan.className = "filter-chip-value";
	valueSpan.textContent = valueLabel;

	chip.append(labelSpan, valueSpan);
	return chip;
}

function chipClearButton(attr, key, value, ariaLabel = "Remove filter") {
	const button = document.createElement("button");
	button.type = "button";
	button.className = "filter-chip-clear";
	button.setAttribute("aria-label", ariaLabel);
	button.textContent = "×";
	button.setAttribute(attr, key);
	if (value !== undefined) {
		button.dataset.value = value;
	}
	return button;
}

function renderFilterGroup(group, options) {
	const shell = filterShell(group.key);
	const selectedCount = state.filters.get(group.key)?.size || 0;
	shell.appendChild(filterTrigger(group.key, group.label, selectedCount));

	const list = document.createElement("div");
	list.className = "filter-menu";
	list.hidden = state.openFilterKey !== group.key;
	for (const item of options) {
		list.appendChild(renderFilterCheckbox(group.key, item, group.label));
	}

	shell.appendChild(list);
	return shell;
}

function filterShell(key) {
	const shell = menuShell();
	shell.dataset.menuShell = key;
	return shell;
}

function menuShell() {
	const shell = document.createElement("div");
	shell.className = "menu-shell";
	shell.dataset.menuShell = "true";
	return shell;
}

function filterTrigger(key, label, selectedCount) {
	const wrapper = document.createElement("div");
	wrapper.className = "filter-trigger";
	wrapper.dataset.selected = String(selectedCount > 0);
	wrapper.appendChild(filterButton(key, filterTriggerLabel(label, selectedCount), selectedCount));
	if (selectedCount > 0) {
		wrapper.appendChild(filterClearButton(key));
	}
	return wrapper;
}

function filterButton(key, label, selectedCount) {
	const button = document.createElement("button");
	button.type = "button";
	button.className = "filter-button";
	button.dataset.filterMenu = key;
	button.textContent = label;
	button.setAttribute("aria-expanded", String(state.openFilterKey === key));
	if (selectedCount > 0) {
		button.dataset.hasClear = "true";
	}
	return button;
}

function filterTriggerLabel(label, selectedCount) {
	return selectedCount === 0 ? label : `${label} (${selectedCount})`;
}

function filterClearButton(key) {
	const button = document.createElement("button");
	button.type = "button";
	button.className = "filter-clear";
	button.dataset.clearFilter = key;
	button.setAttribute("aria-label", "Clear filter");
	button.textContent = "×";
	return button;
}

function renderFilterCheckbox(key, item, groupLabel) {
	const label = document.createElement("label");
	label.className = "filter-option";

	const left = document.createElement("span");
	left.className = "filter-option-left";

	const checkbox = document.createElement("input");
	checkbox.type = "checkbox";
	checkbox.className = "filter-checkbox";
	checkbox.checked = state.filters.get(key)?.has(item.value) || false;
	checkbox.dataset.filterKey = key;
	checkbox.dataset.filterValue = item.value;

	const text = document.createElement("span");
	text.className = "filter-option-label";
	text.textContent = filterOptionLabel(item.value, groupLabel);
	if (item.value === "") {
		text.dataset.empty = "true";
	}

	const count = document.createElement("span");
	count.className = "filter-option-count";
	count.textContent = item.count.toLocaleString();

	left.append(checkbox, text);
	label.append(left, count);
	return label;
}

function renderFlagFilterGroup(flags) {
	const shell = filterShell(FLAG_FILTER_KEY);
	shell.appendChild(filterTrigger(FLAG_FILTER_KEY, "Flags", state.flagFilters.size));

	const list = document.createElement("div");
	list.className = "filter-menu";
	list.hidden = state.openFilterKey !== FLAG_FILTER_KEY;
	for (const item of flags) {
		list.appendChild(renderFlagCheckbox(item));
	}
	shell.appendChild(list);
	return shell;
}

function renderFlagCheckbox(item) {
	const label = document.createElement("label");
	label.className = "filter-option";

	const left = document.createElement("span");
	left.className = "filter-option-left";

	const checkbox = document.createElement("input");
	checkbox.type = "checkbox";
	checkbox.className = "filter-checkbox";
	checkbox.checked = state.flagFilters.has(item.value);
	checkbox.dataset.flagFilter = item.value;

	const text = document.createElement("span");
	text.className = "filter-option-label";
	text.textContent = filterOptionLabel(item.value, "Flags");
	if (item.value === "") {
		text.dataset.empty = "true";
	}

	const count = document.createElement("span");
	count.className = "filter-option-count";
	count.textContent = item.count.toLocaleString();

	left.append(checkbox, text);
	label.append(left, count);
	return label;
}

function renderResults(rows, visibleCount) {
	resultsNode.replaceChildren();
	resultsNode.className = "results-min-height";

	if (state.rows.length === 0) {
		resultsNode.appendChild(emptyState(
			"No IP addresses found",
			"No IPv4 or IPv6 addresses were found in the submitted text.",
		));
		return;
	}

	const wrapper = document.createElement("div");
	wrapper.className = "results-wrapper";
	wrapper.appendChild(renderResultsHeader(visibleCount));

	if (visibleCount === 0) {
		wrapper.appendChild(resultsEmptyMessage(
			"No rows match current filters",
			"Clear filters or choose different values to show results again.",
		));
		resultsNode.appendChild(wrapper);
		return;
	}

	const scroller = document.createElement("div");
	scroller.className = "results-scroller";

	const table = document.createElement("table");
	table.className = "results-table";
	table.append(renderTableColgroup(), renderTableHead(), renderTableBody(rows));

	scroller.appendChild(table);
	wrapper.appendChild(scroller);
	resultsNode.appendChild(wrapper);
}

function renderResultsHeader(visibleCount) {
	const header = document.createElement("div");
	header.className = "results-header";

	const title = document.createElement("h2");
	title.className = "results-title";
	title.textContent = "Lookup results";

	const stats = renderResultsStats(visibleCount);

	header.append(title, stats);
	return header;
}

function renderResultsStats(visibleCount) {
	const stats = state.report.stats;
	const list = document.createElement("dl");
	list.className = "results-stats";
	list.append(
		resultStat("Parsed", stats.total.toLocaleString()),
		resultStat("Unique", stats.unique.toLocaleString()),
	);

	const skipped = stats.unique - stats.reported;
	if (skipped > 0) {
		list.appendChild(resultStat("Skipped", skipped.toLocaleString()));
	}
	list.appendChild(resultStat("Showing", showingStatValue(visibleCount)));
	return list;
}

function resultStat(label, value) {
	const item = document.createElement("div");
	item.className = "results-stat";

	const dt = document.createElement("dt");
	dt.className = "results-stat-label";
	dt.textContent = label;

	const dd = document.createElement("dd");
	dd.className = "results-stat-value";
	dd.textContent = value;

	item.append(dt, dd);
	return item;
}

function showingStatValue(visibleCount) {
	const reported = state.report.stats.reported;
	if (activeFilterCount() > 0) {
		return `${visibleCount.toLocaleString()} / ${reported.toLocaleString()}`;
	}
	return visibleCount.toLocaleString();
}

function resultsEmptyMessage(title, message) {
	const wrapper = document.createElement("div");
	wrapper.className = "results-empty";

	const heading = document.createElement("h3");
	heading.className = "results-empty-title";
	heading.textContent = title;

	const p = document.createElement("p");
	p.className = "results-empty-message";
	p.textContent = message;

	wrapper.append(heading, p);
	return wrapper;
}

function renderTableColgroup() {
	const colgroup = document.createElement("colgroup");
	for (const column of TABLE_COLUMNS) {
		const col = document.createElement("col");
		col.className = `results-column-${column.key}`;
		colgroup.appendChild(col);
	}
	return colgroup;
}

function renderTableHead() {
	const head = document.createElement("thead");
	head.className = "results-head";

	const tr = document.createElement("tr");
	for (const column of TABLE_COLUMNS) {
		const th = document.createElement("th");
		th.scope = "col";
		th.className = "results-heading-cell";
		if (column.sortable) {
			th.setAttribute("aria-sort", ariaSortValue(column.key));
			th.appendChild(sortHeaderButton(column));
		} else {
			th.textContent = column.label;
		}
		tr.appendChild(th);
	}

	head.appendChild(tr);
	return head;
}

function sortHeaderButton(column) {
	const button = document.createElement("button");
	button.type = "button";
	button.className = "sort-header-button";
	button.dataset.sortKey = column.key;
	button.dataset.active = String(state.sort?.key === column.key);
	button.textContent = `${column.label}${sortIndicator(column.key)}`;
	return button;
}

function ariaSortValue(key) {
	if (!state.sort || state.sort.key !== key) {
		return "none";
	}
	return state.sort.direction === SORT_DIRECTION.asc ? "ascending" : "descending";
}

function renderTableBody(rows) {
	const body = document.createElement("tbody");
	body.className = "results-body";

	for (const row of rows) {
		body.appendChild(renderSummaryRow(row));
		if (isExpandable(row) && state.expandedRows.has(row.index)) {
			body.appendChild(renderDetailRow(row));
		}
	}

	return body;
}

function renderSummaryRow(row) {
	const tr = document.createElement("tr");
	tr.className = "result-row";
	tr.dataset.kind = row.kind;
	tr.append(renderIpCell(row, isExpandable(row)), occurrenceCell(row.occurrences));

	if (row.kind === IP_KIND.routable) {
		tr.append(
			valueCell(countryLabel(row)),
			valueCell(row.region),
			valueCell(row.city),
			valueCell(row.asn, "mono"),
			valueCell(row.organization),
			flagsCell(row.flags),
		);
	} else {
		tr.append(
			nodeCell(kindBadge(row.kind)),
			nonRoutableSummaryCell(row),
			flagsCell(row.flags, false),
		);
	}

	return tr;
}

function occurrenceCell(count) {
	const td = document.createElement("td");
	td.className = "result-cell";

	const badge = document.createElement("span");
	badge.className = "occurrence-badge";
	badge.dataset.repeated = String(count > 1);
	badge.textContent = `${count.toLocaleString()}x`;

	td.appendChild(badge);
	return td;
}

function renderIpCell(row, canExpand) {
	const td = document.createElement("td");
	td.className = "result-cell";

	const wrapper = document.createElement("div");
	wrapper.className = "result-ip-content";

	if (canExpand) {
		wrapper.appendChild(expandButton(row));
	} else {
		wrapper.appendChild(expandSpacer());
	}

	const text = document.createElement("div");
	text.className = "result-ip-text";

	const ip = document.createElement("span");
	ip.className = "result-ip-address";
	ip.textContent = row.ip;

	const family = document.createElement("span");
	family.className = "result-ip-family";
	family.textContent = row.family;

	text.append(ip, family);
	wrapper.appendChild(text);
	td.appendChild(wrapper);
	return td;
}

function expandButton(row) {
	const button = document.createElement("button");
	button.type = "button";
	button.className = "row-expand-button";
	button.dataset.expandRow = String(row.index);
	button.setAttribute("aria-expanded", String(state.expandedRows.has(row.index)));
	button.setAttribute("aria-label", state.expandedRows.has(row.index)
		? "Hide details"
		: "Show details");
	button.textContent = state.expandedRows.has(row.index) ? "▲" : "▼";
	return button;
}

function expandSpacer() {
	const spacer = document.createElement("span");
	spacer.className = "row-expand-spacer";
	spacer.setAttribute("aria-hidden", "true");
	return spacer;
}

function valueCell(value, variant = "") {
	const td = document.createElement("td");
	if (!value) {
		td.className = "result-empty-cell";
		td.textContent = "—";
		return td;
	}
	td.className = variant === "mono" ? "result-mono-cell" : "result-text-cell";
	td.textContent = value || "—";
	return td;
}

function nodeCell(node) {
	const td = document.createElement("td");
	td.className = "result-cell";
	td.appendChild(node);
	return td;
}

function nonRoutableSummaryCell(row) {
	const td = document.createElement("td");
	td.className = "result-text-cell";
	td.colSpan = 4;

	const wrapper = document.createElement("span");
	wrapper.className = "non-routable-summary";
	wrapper.appendChild(document.createTextNode(nonRoutableText(row)));

	const rfc = row.entry.specialUse?.rfc;
	if (rfc) {
		const span = document.createElement("span");
		span.className = "non-routable-rfc";
		span.textContent = rfc;
		wrapper.appendChild(span);
	}

	td.appendChild(wrapper);
	return td;
}

function flagsCell(flags, showEmptyMarker = true) {
	const td = document.createElement("td");
	if (flags.length === 0) {
		td.className = "result-empty-cell";
		td.textContent = showEmptyMarker ? "—" : "";
		return td;
	}
	td.className = "result-cell";

	const list = document.createElement("div");
	list.className = "flag-list";
	for (const flag of flags) {
		list.appendChild(flagBadge(flag));
	}
	td.appendChild(list);
	return td;
}

function flagBadge(label) {
	const badge = document.createElement("span");
	badge.className = "flag-badge";
	badge.textContent = label;
	return badge;
}

function kindBadge(kind) {
	const badge = document.createElement("span");
	badge.className = "kind-badge";
	badge.dataset.kind = kind || "Unknown";
	badge.textContent = kind || "Unknown";
	return badge;
}

function nonRoutableText(row) {
	if (row.kind === IP_KIND.specialUse && row.entry.specialUse) {
		return row.entry.specialUse.name;
	}
	if (row.kind === IP_KIND.unallocated) {
		return "Not allocated for public routing.";
	}
	return "Address is not handled as common routable public IP space.";
}

function renderDetailRow(row) {
	const tr = document.createElement("tr");
	tr.className = "detail-row";
	tr.dataset.detailRow = String(row.index);

	const td = document.createElement("td");
	td.className = "detail-cell";
	td.colSpan = TABLE_COLUMNS.length;
	td.appendChild(renderDetails(row));

	tr.appendChild(td);
	return tr;
}

function isExpandable(row) {
	return row.kind === IP_KIND.routable;
}

function renderDetails(row) {
	const groups = detailGroups(row);
	const grid = document.createElement("div");
	grid.className = "detail-grid";
	for (const group of groups) {
		grid.appendChild(renderDetailGroup(group));
	}

	return grid;
}

function renderDetailGroup(group) {
	const section = document.createElement("section");
	section.className = "detail-section";

	const heading = document.createElement("h3");
	heading.className = "detail-heading";
	heading.textContent = group.label;

	const list = document.createElement("dl");
	list.className = "detail-list";
	for (const pair of group.pairs) {
		if (pair.subheading) {
			list.appendChild(renderDetailSubheading(pair.subheading));
			continue;
		}
		list.appendChild(renderDetailPair(pair.label, pair.value));
	}

	section.append(heading, list);
	return section;
}

function renderDetailSubheading(label) {
	const heading = document.createElement("div");
	heading.className = "detail-subheading";
	heading.textContent = label;
	return heading;
}

function renderDetailPair(label, value) {
	const wrapper = document.createElement("div");
	wrapper.className = "detail-item";

	const dt = document.createElement("dt");
	dt.className = "detail-label";
	dt.textContent = label;

	const dd = document.createElement("dd");
	dd.className = "detail-value";
	if (value instanceof Node) {
		dd.appendChild(value);
	} else {
		dd.textContent = value;
	}

	wrapper.append(dt, dd);
	return wrapper;
}

function detailGroups(row) {
	const entry = row.entry;
	const groups = [];

	groups.push(detailGroup("General", [
		detailPair("Address", row.ip),
		detailPair("Version", row.family),
		detailPair("Kind", row.kind),
		detailPair("Input occurrences", occurrenceDetail(row.occurrences)),
		...signalDetails(row),
	]));

	if (hasLocationDetail(row)) {
		groups.push(detailGroup("Location", [
			detailPair("Country", valueOrDash(countryLabel(row))),
			detailPair("Region", valueOrDash(row.region)),
			detailPair("City", valueOrDash(row.city)),
			detailPair("Timezone", entry.geo?.timezone),
			detailPair("Coordinates", coordinates(entry.geo)),
		]));
	}

	groups.push(detailGroup("Network", [
		detailPair("ASN", row.asn),
		detailPair("Organization", row.organization),
		detailPair("Registry handle", entry.asn?.registryHandle),
		detailPair("ASN country", entry.asn?.country),
		detailPair("Category", entry.asn?.category),
		detailPair("Network role", entry.asn?.networkRole),
		detailPair("Network prefix", entry.asn?.network?.prefix),
		detailPair("Network range", networkRange(entry.asn?.network)),
	]));

	return groups.filter((group) => group.pairs.length > 0);
}

function signalDetails(row) {
	const entry = row.entry;
	const pairs = [detailPair("Flags", flagsDetail(row.flags))];
	pairs.push(
		detailPair("Cloud provider", entry.cloud?.provider),
		detailPair("Cloud service", entry.cloud?.service),
		detailPair("Cloud region", entry.cloud?.region),
		detailPair("VPN provider", entry.vpnProvider),
	);

	if (!pairs.some((pair) => hasDetailValue(pair.value))) {
		return [];
	}
	return [detailSubheading("Signals"), ...pairs];
}

function flagsDetail(flags) {
	if (flags.length === 0) {
		return "";
	}

	const fragment = document.createDocumentFragment();
	for (const [index, flag] of flags.entries()) {
		if (index > 0) {
			fragment.appendChild(document.createElement("br"));
		}
		fragment.appendChild(document.createTextNode(flag));
	}
	return fragment;
}

function occurrenceDetail(count) {
	return `${count.toLocaleString()} ${plural(count, "time")}`;
}

function detailPair(label, value) {
	return { label, value };
}

function detailSubheading(label) {
	return { subheading: label };
}

function detailGroup(label, pairs) {
	return { label, pairs: pairs.filter((pair) => pair.subheading || hasDetailValue(pair.value)) };
}

function hasDetailValue(value) {
	return value instanceof Node || Boolean(value);
}

function networkRange(network) {
	return network ? `${network.firstIp} - ${network.lastIp}` : "";
}

function hasLocationDetail(row) {
	return Boolean(
		countryLabel(row) || row.region || row.city || row.entry.geo?.timezone || coordinates(row.entry.geo),
	);
}

function valueOrDash(value) {
	return value || "—";
}

function coordinates(geo) {
	if (!geo?.hasLocation) {
		return "";
	}
	return `${geo.latitude}, ${geo.longitude}`;
}

function countryLabel(row) {
	if (!row.country) {
		return "";
	}
	return row.countryEmoji ? `${row.countryEmoji} ${row.country}` : row.country;
}

function filterRows(rows) {
	return rows.filter((row) => rowMatchesFilters(row));
}

function rowMatchesFilters(row) {
	return rowMatchesFiltersExcept(row, "");
}

function rowMatchesFiltersExcept(row, excludedKey) {
	for (const [key, selectedValues] of state.filters) {
		if (key === excludedKey) {
			continue;
		}
		if (!selectedValues.has(rowValue(row, key))) {
			return false;
		}
	}

	const shouldCheckFlags = excludedKey !== FLAG_FILTER_KEY && state.flagFilters.size > 0;
	if (shouldCheckFlags && !rowMatchesFlagFilters(row)) {
		return false;
	}

	return true;
}

function rowMatchesFlagFilters(row) {
	for (const flag of state.flagFilters) {
		if (flag === "" && row.flags.length === 0) {
			return true;
		}
		if (flag !== "" && row.flagSet.has(flag)) {
			return true;
		}
	}
	return false;
}

function sortRows(rows, sort) {
	const sorted = rows.slice();
	if (!sort) {
		return sorted.sort((a, b) => a.index - b.index);
	}

	sorted.sort((a, b) => {
		const compared = compareRows(a, b, sort.key, sort.direction);
		if (compared !== 0) {
			return compared;
		}
		return a.index - b.index;
	});
	return sorted;
}

function compareRows(a, b, key, direction) {
	if (key === "ip") {
		return compareIP(a.ipSortKey, b.ipSortKey, direction);
	}
	if (key === "occurrences") {
		return compareNumericValues(a.occurrences, b.occurrences, direction);
	}
	if (key === "flags") {
		return compareNullableValues(a.flags.join(" | "), b.flags.join(" | "), direction);
	}
	if (key === "asn") {
		return compareASN(a, b, direction);
	}
	return compareNullableValues(rowValue(a, key), rowValue(b, key), direction);
}

function compareNumericValues(a, b, direction) {
	const compared = compareNumbers(a, b);
	return direction === SORT_DIRECTION.asc ? compared : -compared;
}

function compareIP(a, b, direction) {
	const familyCompared = compareNumbers(a.family, b.family);
	const compared = familyCompared === 0
		? compareBigInts(a.value, b.value)
		: familyCompared;
	return direction === SORT_DIRECTION.asc ? compared : -compared;
}

function compareNumbers(a, b) {
	return a - b;
}

function compareBigInts(a, b) {
	if (a < b) {
		return -1;
	}
	if (a > b) {
		return 1;
	}
	return 0;
}

function compareASN(a, b, direction) {
	const emptyCompared = compareEmpty(a.asn, b.asn);
	if (emptyCompared !== 0) {
		return emptyCompared;
	}
	if (a.asnNumber !== b.asnNumber) {
		return direction === SORT_DIRECTION.asc
			? a.asnNumber - b.asnNumber
			: b.asnNumber - a.asnNumber;
	}
	return compareNullableValues(a.asn, b.asn, direction);
}

function compareNullableValues(a, b, direction) {
	const emptyCompared = compareEmpty(a, b);
	if (emptyCompared !== 0) {
		return emptyCompared;
	}
	const compared = collator.compare(a, b);
	return direction === SORT_DIRECTION.asc ? compared : -compared;
}

function compareEmpty(a, b) {
	const aEmpty = a === "";
	const bEmpty = b === "";
	if (aEmpty && !bEmpty) {
		return 1;
	}
	if (!aEmpty && bEmpty) {
		return -1;
	}
	return 0;
}

function rowValue(row, key) {
	return row[key] || "";
}

function toggleFilterValue(key, value, checked) {
	const values = new Set(state.filters.get(key) || []);
	if (checked) {
		values.add(value);
	} else {
		values.delete(value);
	}

	if (values.size === 0) {
		state.filters.delete(key);
	} else {
		state.filters.set(key, values);
	}
	renderApp();
}

function toggleFlagFilter(flag, checked) {
	if (checked) {
		state.flagFilters.add(flag);
	} else {
		state.flagFilters.delete(flag);
	}
	renderApp();
}

function clearFilters() {
	state.filters = new Map();
	state.flagFilters = new Set();
	state.openFilterKey = null;
	renderApp();
}

function clearFilter(key) {
	if (key === FLAG_FILTER_KEY) {
		state.flagFilters = new Set();
	} else {
		state.filters.delete(key);
	}
	state.openFilterKey = null;
	renderApp();
}

function clearFilterValue(key, value) {
	const values = new Set(state.filters.get(key) || []);
	values.delete(value);
	if (values.size === 0) {
		state.filters.delete(key);
	} else {
		state.filters.set(key, values);
	}
	state.openFilterKey = null;
	renderApp();
}

function clearFlagValue(flag) {
	state.flagFilters.delete(flag);
	state.openFilterKey = null;
	renderApp();
}

function toggleFilterMenu(key) {
	state.openFilterKey = state.openFilterKey === key ? null : key;
	renderApp();
}

function activeFilterCount() {
	let count = state.flagFilters.size;
	for (const values of state.filters.values()) {
		count += values.size;
	}
	return count;
}

function filterOptions(rows, key) {
	const counts = new Map();
	for (const row of rows) {
		if (!rowMatchesFiltersExcept(row, key)) {
			continue;
		}
		const option = rowValue(row, key);
		counts.set(option, (counts.get(option) || 0) + 1);
	}
	addSelectedFilterOptions(counts, key);
	return countedOptions(counts);
}

function flagOptions(rows) {
	const counts = new Map();
	for (const row of rows) {
		if (!rowMatchesFiltersExcept(row, FLAG_FILTER_KEY)) {
			continue;
		}
		if (row.flags.length === 0) {
			counts.set("", (counts.get("") || 0) + 1);
			continue;
		}
		for (const flag of row.flags) {
			counts.set(flag, (counts.get(flag) || 0) + 1);
		}
	}
	addSelectedFlagOptions(counts);
	return countedOptions(counts);
}

function addSelectedFilterOptions(counts, key) {
	const selectedValues = state.filters.get(key);
	if (!selectedValues) {
		return;
	}
	for (const value of selectedValues) {
		if (!counts.has(value)) {
			counts.set(value, 0);
		}
	}
}

function addSelectedFlagOptions(counts) {
	for (const flag of state.flagFilters) {
		if (!counts.has(flag)) {
			counts.set(flag, 0);
		}
	}
}

function countedOptions(counts) {
	return Array.from(counts, ([itemValue, count]) => ({ value: itemValue, count }))
		.sort((a, b) => compareFilterOptions(a.value, b.value));
}

function compareFilterOptions(a, b) {
	const emptyCompared = compareEmptyFirst(a, b);
	if (emptyCompared !== 0) {
		return emptyCompared;
	}
	return collator.compare(a, b);
}

function compareEmptyFirst(a, b) {
	const aEmpty = a === "";
	const bEmpty = b === "";
	if (aEmpty && !bEmpty) {
		return -1;
	}
	if (!aEmpty && bEmpty) {
		return 1;
	}
	return 0;
}

function filterOptionLabel(value, groupLabel) {
	if (value !== "") {
		return value;
	}
	return `No ${groupLabel}`;
}

function cycleSort(key) {
	if (!state.sort || state.sort.key !== key) {
		state.sort = { key, direction: defaultSortDirection(key) };
	} else if (state.sort.direction === defaultSortDirection(key)) {
		state.sort = { key, direction: alternateSortDirection(key) };
	} else {
		state.sort = null;
	}
	renderApp();
}

function defaultSortDirection(key) {
	return key === "occurrences" ? SORT_DIRECTION.desc : SORT_DIRECTION.asc;
}

function alternateSortDirection(key) {
	return defaultSortDirection(key) === SORT_DIRECTION.asc
		? SORT_DIRECTION.desc
		: SORT_DIRECTION.asc;
}

function sortIndicator(key) {
	if (!state.sort || state.sort.key !== key) {
		return "";
	}
	return state.sort.direction === SORT_DIRECTION.asc ? " ↑" : " ↓";
}

function toggleExpandedRow(index) {
	if (state.expandedRows.has(index)) {
		state.expandedRows.delete(index);
		renderApp();
	} else {
		state.expandedRows.add(index);
		renderApp();
		revealDetailRow(index);
	}
}

function showError(message) {
	statusNode.className = "lookup-status";
	statusNode.dataset.status = "error";
	statusNode.textContent = message;
}

function hideStatus() {
	statusNode.className = "lookup-status";
	statusNode.dataset.status = "idle";
	statusNode.textContent = "";
}

function setBusy(isBusy) {
	state.isBusy = isBusy;
	submitButton.textContent = isBusy ? "Looking up..." : "Lookup";
	form.setAttribute("aria-busy", String(isBusy));
	updateFormState();
}

function updateFormState() {
	updateInputSize();
	submitButton.disabled = state.isBusy || inputNode.value.trim() === "";
}

function updateInputSize() {
	const used = formatBytes(byteLength(inputNode.value));
	const limit = formatBytes(maxLookupBodyBytes());
	inputSizeNode.textContent = `${used} / ${limit}`;
}

function scrollToLookupOutput() {
	const target = controlsNode.hidden ? resultsNode : controlsNode;
	target.scrollIntoView({ behavior: scrollBehavior(), block: "start" });
}

function revealDetailRow(index) {
	const row = resultsNode.querySelector(`[data-detail-row="${index}"]`);
	row?.scrollIntoView({ behavior: scrollBehavior(), block: "nearest", inline: "nearest" });
}

function scrollBehavior() {
	return window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth";
}

function emptyState(title, message, tone = "") {
	const wrapper = document.createElement("div");
	wrapper.className = tone === "quiet" ? "empty-state empty-state-quiet" : "empty-state";

	const heading = document.createElement("h2");
	heading.className = "empty-state-title";
	heading.textContent = title;

	wrapper.appendChild(heading);

	if (message) {
		const p = document.createElement("p");
		p.className = "empty-state-message";
		p.textContent = message;
		wrapper.appendChild(p);
	}
	return wrapper;
}

function buttonElement(label, className) {
	const button = document.createElement("button");
	button.type = "button";
	button.className = className;
	button.textContent = label;
	return button;
}

function formatBytes(bytes) {
	if (bytes < 1024) {
		return `${bytes} B`;
	}
	return `${Math.ceil(bytes / 1024)} KiB`;
}
