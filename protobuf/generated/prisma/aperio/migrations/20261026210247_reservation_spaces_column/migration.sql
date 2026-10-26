/*
  Warnings:

  - You are about to drop the `reservation_spaces` table. If the table is not empty, all the data it contains will be lost.

*/
-- DropForeignKey
ALTER TABLE "stay"."reservation_spaces" DROP CONSTRAINT "reservation_spaces_reservation_id_fkey";

-- DropForeignKey
ALTER TABLE "stay"."reservation_spaces" DROP CONSTRAINT "reservation_spaces_space_id_fkey";

-- AlterTable
ALTER TABLE "stay"."reservations" ADD COLUMN     "spaces" TEXT[];

-- DropTable
DROP TABLE "stay"."reservation_spaces";

-- CreateIndex
CREATE INDEX "reservations_spaces_idx" ON "stay"."reservations" USING GIN ("spaces");
