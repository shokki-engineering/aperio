-- AlterTable
ALTER TABLE "venue"."resource" ADD COLUMN     "invoice_prefix" TEXT;

-- CreateIndex
CREATE UNIQUE INDEX "venue_resource_invoice_prefix_key" ON "venue"."resource"("invoice_prefix");

