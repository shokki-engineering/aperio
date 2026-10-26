-- AlterTable
ALTER TABLE "crm"."contacts" ADD COLUMN     "phone_numbers" TEXT[];

-- CreateIndex
CREATE INDEX "contacts_phone_numbers_idx" ON "crm"."contacts" USING GIN ("phone_numbers");

