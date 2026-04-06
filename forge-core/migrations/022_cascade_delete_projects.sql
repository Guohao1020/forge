-- Add ON DELETE CASCADE to all foreign keys referencing engine.projects(id)
-- so that deleting a project automatically cleans up all child records.
-- Uses DO blocks to skip tables that may not exist yet.

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='tasks') THEN
    ALTER TABLE engine.tasks DROP CONSTRAINT IF EXISTS tasks_project_id_fkey;
    ALTER TABLE engine.tasks ADD CONSTRAINT tasks_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='pipeline' AND table_name='environments') THEN
    ALTER TABLE pipeline.environments DROP CONSTRAINT IF EXISTS environments_project_id_fkey;
    ALTER TABLE pipeline.environments ADD CONSTRAINT environments_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='pipeline' AND table_name='deploy_records') THEN
    ALTER TABLE pipeline.deploy_records DROP CONSTRAINT IF EXISTS deploy_records_project_id_fkey;
    ALTER TABLE pipeline.deploy_records ADD CONSTRAINT deploy_records_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='pipeline' AND table_name='preview_environments') THEN
    ALTER TABLE pipeline.preview_environments DROP CONSTRAINT IF EXISTS preview_environments_project_id_fkey;
    ALTER TABLE pipeline.preview_environments ADD CONSTRAINT preview_environments_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='project_profiles') THEN
    ALTER TABLE engine.project_profiles DROP CONSTRAINT IF EXISTS project_profiles_project_id_fkey;
    ALTER TABLE engine.project_profiles ADD CONSTRAINT project_profiles_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='pipeline' AND table_name='artifacts') THEN
    ALTER TABLE pipeline.artifacts DROP CONSTRAINT IF EXISTS artifacts_project_id_fkey;
    ALTER TABLE pipeline.artifacts ADD CONSTRAINT artifacts_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='entropy_scans') THEN
    ALTER TABLE engine.entropy_scans DROP CONSTRAINT IF EXISTS entropy_scans_project_id_fkey;
    ALTER TABLE engine.entropy_scans ADD CONSTRAINT entropy_scans_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='entropy_configs') THEN
    ALTER TABLE engine.entropy_configs DROP CONSTRAINT IF EXISTS entropy_configs_project_id_fkey;
    ALTER TABLE engine.entropy_configs ADD CONSTRAINT entropy_configs_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='webhooks') THEN
    ALTER TABLE engine.webhooks DROP CONSTRAINT IF EXISTS webhooks_project_id_fkey;
    ALTER TABLE engine.webhooks ADD CONSTRAINT webhooks_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES engine.projects(id) ON DELETE CASCADE;
  END IF;
END $$;

-- Also fix cascading deletes on task child tables (conversations, task_nodes, etc.)
-- so deleting a project cascades through tasks to their children.

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='conversations') THEN
    ALTER TABLE engine.conversations DROP CONSTRAINT IF EXISTS conversations_task_id_fkey;
    ALTER TABLE engine.conversations ADD CONSTRAINT conversations_task_id_fkey
        FOREIGN KEY (task_id) REFERENCES engine.tasks(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='task_nodes') THEN
    ALTER TABLE engine.task_nodes DROP CONSTRAINT IF EXISTS task_nodes_task_id_fkey;
    ALTER TABLE engine.task_nodes ADD CONSTRAINT task_nodes_task_id_fkey
        FOREIGN KEY (task_id) REFERENCES engine.tasks(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='review_results') THEN
    ALTER TABLE engine.review_results DROP CONSTRAINT IF EXISTS review_results_task_id_fkey;
    ALTER TABLE engine.review_results ADD CONSTRAINT review_results_task_id_fkey
        FOREIGN KEY (task_id) REFERENCES engine.tasks(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='engine' AND table_name='test_results') THEN
    ALTER TABLE engine.test_results DROP CONSTRAINT IF EXISTS test_results_task_id_fkey;
    ALTER TABLE engine.test_results ADD CONSTRAINT test_results_task_id_fkey
        FOREIGN KEY (task_id) REFERENCES engine.tasks(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='pipeline' AND table_name='artifacts') THEN
    ALTER TABLE pipeline.artifacts DROP CONSTRAINT IF EXISTS artifacts_task_id_fkey;
    ALTER TABLE pipeline.artifacts ADD CONSTRAINT artifacts_task_id_fkey
        FOREIGN KEY (task_id) REFERENCES engine.tasks(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='pipeline' AND table_name='preview_environments') THEN
    ALTER TABLE pipeline.preview_environments DROP CONSTRAINT IF EXISTS preview_environments_task_id_fkey;
    ALTER TABLE pipeline.preview_environments ADD CONSTRAINT preview_environments_task_id_fkey
        FOREIGN KEY (task_id) REFERENCES engine.tasks(id) ON DELETE CASCADE;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='pipeline' AND table_name='deploy_records') THEN
    ALTER TABLE pipeline.deploy_records DROP CONSTRAINT IF EXISTS deploy_records_environment_id_fkey;
    ALTER TABLE pipeline.deploy_records ADD CONSTRAINT deploy_records_environment_id_fkey
        FOREIGN KEY (environment_id) REFERENCES pipeline.environments(id) ON DELETE CASCADE;
  END IF;
END $$;
