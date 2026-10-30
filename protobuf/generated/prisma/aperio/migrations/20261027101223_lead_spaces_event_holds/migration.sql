-- AlterTable
ALTER TABLE "event"."resource" ADD COLUMN     "freebusy_holds" TEXT[];

-- AlterTable
ALTER TABLE "sales"."leads" ADD COLUMN     "spaces" TEXT[];

-- CreateIndex
CREATE INDEX "resource_freebusy_holds_idx" ON "event"."resource" USING GIN ("freebusy_holds");

-- CreateIndex
CREATE INDEX "service_requests_state_create_time_idx" ON "operations"."service_requests"("state", "create_time");

-- CreateIndex
CREATE INDEX "leads_spaces_idx" ON "sales"."leads" USING GIN ("spaces");
