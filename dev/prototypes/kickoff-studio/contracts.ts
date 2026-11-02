// Prototype-only contracts. No real API or domain persistence is implemented here.
export type StudioColumn = {
            key: string;
            label: string;
            /** @enum {string} */
            kind: "text" | "number" | "positive" | "integer";
            required: boolean;
        };

export type StudioModel = {
            id: string;
            name: string;
            /** @enum {string} */
            group: "facility" | "cabling" | "network_power";
            purpose: string;
            step_ids: number[];
            columns: StudioColumn[];
            rows: string[][];
            version: number;
            /** @enum {string} */
            status: "draft" | "adopted";
            source: string;
        };

export type StudioSource = {
            id: string;
            name: string;
            /** @enum {string} */
            kind: "example" | "user_metadata";
            step_ids: number[];
            status: string;
        };

export type StudioIssue = {
            id: string;
            step_id: number;
            title: string;
            detail: string;
            action_label: string;
            model_id: string;
            /** @enum {string} */
            severity: "BLOCK" | "INFO";
        };

export type StudioAsset = {
            id: string;
            label: string;
            serial: string;
            model: string;
            rack: string;
            position: string;
            source: string;
        };

export type StudioStep = {
            id: number;
            name: string;
            summary: string;
            /** @enum {string} */
            state: "DETECTED" | "NEEDS_INPUT" | "READY" | "PUBLISHED" | "STALE";
            input_labels: string[];
            outputs: string[];
            asset_count: number;
            issue_count: number;
        };

export type StudioCalculation = {
            route_m: number;
            leads_m: number;
            slack_percent: number;
            required_m: number;
            stock_m: number;
            cable_count: number;
            optic_count: number;
            rule_version: number;
        };

export type StudioBenchmark = {
            model_id: string;
            provider_ref: string;
            prompt_version: string;
            run_count: number;
            /** @enum {string} */
            status: "NOT_RUN";
            checks: string[];
        };

export type StudioSnapshot = {
            termination: StudioTermination;
            project_id: string;
            project_name: string;
            revision: number;
            /** @constant */
            simulation_only: true;
            analyzed: boolean;
            identity_confirmed: boolean;
            review_revision: number | null;
            published_revision: number | null;
            last_published_revision: number | null;
            models: StudioModel[];
            sources: StudioSource[];
            assets: StudioAsset[];
            steps: StudioStep[];
            issues: StudioIssue[];
            calculation: StudioCalculation | null;
            benchmark: StudioBenchmark;
            events: string[];
        };

export type StudioAction = {
            b_mac?: string;
            expected_revision: number;
            /** @enum {string} */
            action: "analyze_example" | "confirm_identity" | "adopt_models" | "calculate" | "review" | "publish";
            reason: string;
        };

export type StudioModelInput = {
            expected_revision: number;
            rows: string[][];
            /** @enum {string} */
            decision: "save" | "adopt";
            reason: string;
        };

export type StudioSourceInput = {
            expected_revision: number;
            filenames: string[];
        };

export type StudioBenchmarkInput = {
            expected_revision: number;
            provider_ref: string;
        };

export type StudioTermination = {
            a_sn: string;
            a_mac: string;
            a_port: string;
            b_sn: string;
            b_rack_position: string;
            b_mac: string;
            b_port: string;
            termination_type: string;
        };
